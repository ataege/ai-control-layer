package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"starter/services/gateway/internal/health"
	"starter/services/gateway/internal/logging"
)

const testServiceToken = "test-token-0123456789abcdef0123456789abcdef"

// fakePinger stands in for *pgxpool.Pool.
type fakePinger struct{ err error }

func (pinger fakePinger) Ping(context.Context) error { return pinger.err }

func newTestHandler(pinger health.Pinger, logger *slog.Logger) http.Handler {
	return NewHandler(Options{
		Logger:       logger,
		Health:       health.Handler{Database: pinger, DatabaseTimeout: time.Second, Logger: logger},
		ServiceToken: logging.NewSecret(testServiceToken),
	})
}

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// decodeStrict fails the test when the body has fields the DTO does not know.
func decodeStrict(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	strictDecoder := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes()))
	strictDecoder.DisallowUnknownFields()
	if err := strictDecoder.Decode(target); err != nil {
		t.Fatalf("body %q does not match the expected shape: %v", recorder.Body.String(), err)
	}
}

// assertErrorEnvelope checks the shared ErrorResponse contract.
func assertErrorEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode, wantPath string) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body %q)", recorder.Code, wantStatus, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", contentType)
	}
	var envelope health.ErrorResponse
	decodeStrict(t, recorder, &envelope)
	if envelope.Error.Code != wantCode || envelope.Error.Message == "" {
		t.Errorf("error = %+v, want code %q with a message", envelope.Error, wantCode)
	}
	if envelope.StatusCode != wantStatus {
		t.Errorf("statusCode = %d, want %d", envelope.StatusCode, wantStatus)
	}
	if envelope.RequestID == "" || envelope.RequestID != recorder.Header().Get(requestIDHeader) {
		t.Errorf("requestId = %q, header = %q", envelope.RequestID, recorder.Header().Get(requestIDHeader))
	}
	if _, err := time.Parse(time.RFC3339Nano, envelope.Timestamp); err != nil {
		t.Errorf("timestamp %q is not ISO 8601: %v", envelope.Timestamp, err)
	}
	if envelope.Path != wantPath {
		t.Errorf("path = %q, want %q", envelope.Path, wantPath)
	}
}

func TestLivenessIgnoresTheDatabase(t *testing.T) {
	handler := newTestHandler(fakePinger{err: errors.New("database is down")}, discardLogger())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body health.LivenessResponse
	decodeStrict(t, recorder, &body)
	if body != (health.LivenessResponse{Status: "ok", Service: "gateway"}) {
		t.Errorf("body = %+v", body)
	}
}

func TestReadiness(t *testing.T) {
	const driverErrorDetail = "failed to connect to `user=internal-user database=internal-db`: 10.1.2.3:5432"

	tests := []struct {
		name       string
		pingError  error
		wantStatus int
		wantBody   health.ReadinessResponse
	}{
		{
			name:       "healthy database",
			wantStatus: http.StatusOK,
			wantBody: health.ReadinessResponse{
				Status: "ok", Service: "gateway",
				Checks: health.ReadinessChecks{Database: health.DependencyCheck{Status: "up"}},
			},
		},
		{
			name:       "failing database",
			pingError:  errors.New(driverErrorDetail),
			wantStatus: http.StatusServiceUnavailable,
			wantBody: health.ReadinessResponse{
				Status: "unavailable", Service: "gateway",
				Checks: health.ReadinessChecks{Database: health.DependencyCheck{Status: "down", Message: "database unreachable"}},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var logOutput bytes.Buffer
			logger := logging.New(&logOutput, slog.LevelDebug)
			handler := newTestHandler(fakePinger{err: testCase.pingError}, logger)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.wantStatus)
			}
			var body health.ReadinessResponse
			decodeStrict(t, recorder, &body)
			if body != testCase.wantBody {
				t.Errorf("body = %+v, want %+v", body, testCase.wantBody)
			}
			for _, internalDetail := range []string{"internal-user", "internal-db", "10.1.2.3"} {
				if strings.Contains(recorder.Body.String(), internalDetail) {
					t.Errorf("response leaks %q: %s", internalDetail, recorder.Body.String())
				}
			}
			// The detail is still available to operators in the server log.
			if testCase.pingError != nil && !strings.Contains(logOutput.String(), "internal-user") {
				t.Errorf("server log is missing the failure detail: %s", logOutput.String())
			}
		})
	}
}

// fakeWorker stands in for the worker service in readiness.
type fakeWorker struct{ ready bool }

func (worker fakeWorker) Ready() bool { return worker.ready }

func TestReadinessCoversTheWorker(t *testing.T) {
	tests := []struct {
		name       string
		pingError  error
		worker     fakeWorker
		wantStatus int
		wantBody   health.ReadinessResponse
	}{
		{
			name: "worker running", worker: fakeWorker{ready: true}, wantStatus: http.StatusOK,
			wantBody: health.ReadinessResponse{Status: "ok", Service: "gateway",
				Checks: health.ReadinessChecks{Database: health.DependencyCheck{Status: "up"}}},
		},
		{
			// The schema has no worker field: the database check stays truthful, the status drops.
			name: "worker not running", worker: fakeWorker{ready: false}, wantStatus: http.StatusServiceUnavailable,
			wantBody: health.ReadinessResponse{Status: "unavailable", Service: "gateway",
				Checks: health.ReadinessChecks{Database: health.DependencyCheck{Status: "up"}}},
		},
		{
			name: "worker not running and database down", pingError: errors.New("down"), worker: fakeWorker{ready: false},
			wantStatus: http.StatusServiceUnavailable,
			wantBody: health.ReadinessResponse{Status: "unavailable", Service: "gateway",
				Checks: health.ReadinessChecks{Database: health.DependencyCheck{Status: "down", Message: "database unreachable"}}},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var logOutput bytes.Buffer
			logger := logging.New(&logOutput, slog.LevelDebug)
			handler := NewHandler(Options{
				Logger: logger,
				Health: health.Handler{Database: fakePinger{err: testCase.pingError}, DatabaseTimeout: time.Second,
					Logger: logger, Worker: testCase.worker},
				ServiceToken: logging.NewSecret(testServiceToken),
			})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.wantStatus)
			}
			var body health.ReadinessResponse
			decodeStrict(t, recorder, &body)
			if body != testCase.wantBody {
				t.Errorf("body = %+v, want %+v", body, testCase.wantBody)
			}
			if testCase.pingError == nil && !testCase.worker.ready && !strings.Contains(logOutput.String(), "worker loop not running") {
				t.Errorf("the worker failure is not in the server log: %s", logOutput.String())
			}
		})
	}
}

// slowPinger blocks until the readiness deadline cancels the context.
type slowPinger struct{}

func (slowPinger) Ping(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestReadinessIsBoundedByTheDatabaseTimeout(t *testing.T) {
	logger := discardLogger()
	handler := NewHandler(Options{
		Logger:       logger,
		Health:       health.Handler{Database: slowPinger{}, DatabaseTimeout: 50 * time.Millisecond, Logger: logger},
		ServiceToken: logging.NewSecret(testServiceToken),
	})
	startedAt := time.Now()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Errorf("readiness took %s, want it bounded by the timeout", elapsed)
	}
}

func TestInternalPingServiceToken(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		wantStatus    int
	}{
		{"missing header", "", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong-token-0123456789abcdef0123456789abcdef", http.StatusUnauthorized},
		{"prefix of token", "Bearer " + testServiceToken[:10], http.StatusUnauthorized},
		{"token plus suffix", "Bearer " + testServiceToken + "x", http.StatusUnauthorized},
		{"wrong scheme", "Basic " + testServiceToken, http.StatusUnauthorized},
		{"no scheme", testServiceToken, http.StatusUnauthorized},
		{"empty bearer", "Bearer ", http.StatusUnauthorized},
		{"extra segment", "Bearer " + testServiceToken + " extra", http.StatusUnauthorized},
		{"valid token", "Bearer " + testServiceToken, http.StatusOK},
		{"scheme is case-insensitive", "bearer " + testServiceToken, http.StatusOK},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// A failing pinger proves the ping route never touches the database.
			handler := newTestHandler(fakePinger{err: errors.New("database is down")}, discardLogger())
			request := httptest.NewRequest(http.MethodGet, "/internal/ping", nil)
			if testCase.authorization != "" {
				request.Header.Set("Authorization", testCase.authorization)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if testCase.wantStatus == http.StatusOK {
				if recorder.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200", recorder.Code)
				}
				var body health.GatewayPingResponse
				decodeStrict(t, recorder, &body)
				if body != (health.GatewayPingResponse{Status: "ok", Service: "gateway"}) {
					t.Errorf("body = %+v", body)
				}
				return
			}
			assertErrorEnvelope(t, recorder, http.StatusUnauthorized, "unauthorized", "/internal/ping")
			if recorder.Header().Get("WWW-Authenticate") == "" {
				t.Error("missing WWW-Authenticate header")
			}
			if strings.Contains(recorder.Body.String(), testServiceToken) {
				t.Error("401 body echoes the service token")
			}
		})
	}
}

func TestEmptyExpectedTokenRejectsEverything(t *testing.T) {
	protected := RequireServiceToken(logging.NewSecret(""))(http.HandlerFunc(health.Handler{}.Ping))
	for _, authorization := range []string{"", "Bearer ", "Bearer anything"} {
		request := httptest.NewRequest(http.MethodGet, "/internal/ping", nil)
		if authorization != "" {
			request.Header.Set("Authorization", authorization)
		}
		recorder := httptest.NewRecorder()
		protected.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("Authorization %q: status = %d, want 401", authorization, recorder.Code)
		}
	}
}

func TestUnknownRoutesUseTheErrorEnvelope(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantCode   string
		wantPath   string
	}{
		{"unknown path", http.MethodGet, "/nope", http.StatusNotFound, "not_found", "/nope"},
		{"root", http.MethodGet, "/", http.StatusNotFound, "not_found", "/"},
		{"trailing slash", http.MethodGet, "/health/live/", http.StatusNotFound, "not_found", "/health/live/"},
		{"unclean path is not redirected", http.MethodGet, "//health/live", http.StatusNotFound, "not_found", "//health/live"},
		{"dot segments are not redirected", http.MethodGet, "/health/../health/live", http.StatusNotFound, "not_found", "/health/../health/live"},
		{"wrong method on live", http.MethodPost, "/health/live", http.StatusMethodNotAllowed, "method_not_allowed", "/health/live"},
		{"wrong method on ping", http.MethodDelete, "/internal/ping", http.StatusMethodNotAllowed, "method_not_allowed", "/internal/ping"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			handler := newTestHandler(fakePinger{}, discardLogger())
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(testCase.method, testCase.target, nil))

			assertErrorEnvelope(t, recorder, testCase.wantStatus, testCase.wantCode, testCase.wantPath)
			if testCase.wantStatus == http.StatusMethodNotAllowed && recorder.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("Allow = %q, want \"GET, HEAD\"", recorder.Header().Get("Allow"))
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	tests := []struct {
		name       string
		inbound    string
		wantReused bool
	}{
		{"generated when missing", "", false},
		{"safe id is reused", "abc-123_DEF.9", true},
		{"64 characters are reused", strings.Repeat("a", 64), true},
		{"65 characters are replaced", strings.Repeat("a", 65), false},
		{"unsafe characters are replaced", "evil\"id{}", false},
		{"spaces are replaced", "two words", false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var logOutput bytes.Buffer
			handler := newTestHandler(fakePinger{}, logging.New(&logOutput, slog.LevelInfo))

			request := httptest.NewRequest(http.MethodGet, "/internal/ping?debug=query-string-value", nil)
			request.Header.Set("Authorization", "Bearer "+testServiceToken)
			if testCase.inbound != "" {
				request.Header.Set("x-request-id", testCase.inbound)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			responseID := recorder.Header().Get("x-request-id")
			if testCase.wantReused && responseID != testCase.inbound {
				t.Errorf("request id = %q, want the inbound %q", responseID, testCase.inbound)
			}
			if !testCase.wantReused && (responseID == testCase.inbound || !isSafeRequestID(responseID)) {
				t.Errorf("request id = %q, want a freshly generated safe id", responseID)
			}

			var accessLine map[string]any
			if err := json.Unmarshal(logOutput.Bytes(), &accessLine); err != nil {
				t.Fatalf("access log is not one JSON line: %v (%q)", err, logOutput.String())
			}
			if accessLine["request_id"] != responseID {
				t.Errorf("logged request_id = %v, want %q", accessLine["request_id"], responseID)
			}
			// Tokens, query strings and rejected ids must stay out of the log.
			for _, forbidden := range []string{testServiceToken, "query-string-value", "Bearer"} {
				if strings.Contains(logOutput.String(), forbidden) {
					t.Errorf("access log contains %q: %s", forbidden, logOutput.String())
				}
			}
			if !testCase.wantReused && testCase.inbound != "" && strings.Contains(logOutput.String(), testCase.inbound) {
				t.Errorf("access log contains the rejected request id: %s", logOutput.String())
			}
		})
	}
}

func TestPanicRecoveryReturnsTheErrorEnvelope(t *testing.T) {
	var logOutput bytes.Buffer
	logger := logging.New(&logOutput, slog.LevelInfo)
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom with internal detail") })
	handler := Chain(panicking, RequestID(logger), AccessLog(logger), Recover(logger))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/any", nil))

	assertErrorEnvelope(t, recorder, http.StatusInternalServerError, "internal_error", "/any")
	if strings.Contains(recorder.Body.String(), "boom") || strings.Contains(recorder.Body.String(), "goroutine") {
		t.Errorf("500 body leaks panic details: %s", recorder.Body.String())
	}
	if !strings.Contains(logOutput.String(), "boom with internal detail") || !strings.Contains(logOutput.String(), `"status":500`) {
		t.Errorf("panic or access line missing from the log: %s", logOutput.String())
	}
}

func TestRecoverRepanicsAbortHandler(t *testing.T) {
	aborting := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
	handler := Recover(discardLogger())(aborting)
	defer func() {
		if recover() != http.ErrAbortHandler {
			t.Error("ErrAbortHandler was swallowed")
		}
	}()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
