package httpserver

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"starter/services/gateway/internal/health"
	"starter/services/gateway/internal/logging"
	"starter/services/gateway/internal/operatorcontext"
)

const testSigningKey = "httpserver-test-signing-key-0123456789abcdef"

const testOrganizationID = "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01"

// signOperatorContext signs a token the way the API does, issued now with a one-minute life.
func signOperatorContext(t *testing.T, key, tokenID string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "HS256"})
	now := time.Now()
	claims, _ := json.Marshal(map[string]any{
		"ctx": map[string]any{"userId": "2c3d4e5f-6a7b-4c8d-9e0f-1a2b3c4d5e6f",
			"organizationId": testOrganizationID, "roles": []string{"operator"}},
		"iat": now.Unix(), "exp": now.Add(time.Minute).Unix(),
		"aud": "gateway", "iss": "gateway-client", "jti": tokenID,
	})
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// commandProbe is a test command that decodes a small body and echoes what it was given.
type commandProbe struct {
	calls int
}

type probeBody struct {
	Name string `json:"name"`
}

func (probe *commandProbe) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	probe.calls++
	operator, verified := operatorcontext.FromContext(request.Context())
	if !verified {
		WriteError(responseWriter, request, http.StatusInternalServerError, "internal_error", "No operator.")
		return
	}
	var body probeBody
	if !DecodeJSONBody(responseWriter, request, 64, &body) {
		return
	}
	health.WriteJSON(responseWriter, http.StatusOK, map[string]string{
		"thing": request.PathValue("thingID"), "organization": operator.OrganizationID, "name": body.Name,
	})
}

func newCommandHandler(t *testing.T, logger *slog.Logger, withVerifier bool) (http.Handler, *commandProbe) {
	t.Helper()
	var verifier *operatorcontext.Verifier
	if withVerifier {
		var err error
		verifier, err = operatorcontext.NewVerifier(logging.NewSecret(testSigningKey))
		if err != nil {
			t.Fatalf("NewVerifier: %v", err)
		}
	}
	probe := &commandProbe{}
	return NewHandler(Options{
		Logger:           logger,
		Health:           health.Handler{Database: fakePinger{}, DatabaseTimeout: time.Second, Logger: logger},
		ServiceToken:     logging.NewSecret(testServiceToken),
		OperatorContext:  verifier,
		InternalCommands: []InternalCommand{{Pattern: "POST /internal/things/{thingID}", Handler: probe}},
	}), probe
}

func commandRequest(body string, headers map[string][]string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/internal/things/thing-7", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for name, values := range headers {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	return request
}

func TestInternalCommandAcceptsVerifiedOperator(t *testing.T) {
	handler, probe := newCommandHandler(t, discardLogger(), true)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, commandRequest(`{"name":"atlas"}`, map[string][]string{
		"Authorization":            {"Bearer " + testServiceToken},
		operatorcontext.HeaderName: {signOperatorContext(t, testSigningKey, "accept-1")},
	}))
	if recorder.Code != http.StatusOK || probe.calls != 1 {
		t.Fatalf("status = %d, calls = %d, body %q", recorder.Code, probe.calls, recorder.Body.String())
	}
	var echoed map[string]string
	decodeStrict(t, recorder, &echoed)
	if echoed["thing"] != "thing-7" || echoed["organization"] != testOrganizationID || echoed["name"] != "atlas" {
		t.Errorf("handler received %v", echoed)
	}
}

func TestInternalCommandRejectsBeforeTheHandler(t *testing.T) {
	validToken := signOperatorContext(t, testSigningKey, "reject-valid")
	cases := map[string]map[string][]string{
		"no credentials":         {},
		"service token alone":    {"Authorization": {"Bearer " + testServiceToken}},
		"operator context alone": {operatorcontext.HeaderName: {validToken}},
		"wrong service token": {"Authorization": {"Bearer " + strings.Repeat("x", 40)},
			operatorcontext.HeaderName: {validToken}},
		"context signed with another key": {"Authorization": {"Bearer " + testServiceToken},
			operatorcontext.HeaderName: {signOperatorContext(t, "another-signing-key-0123456789abcdefgh", "reject-forged")}},
		"two context headers": {"Authorization": {"Bearer " + testServiceToken},
			operatorcontext.HeaderName: {validToken, signOperatorContext(t, testSigningKey, "reject-second")}},
		"garbage context": {"Authorization": {"Bearer " + testServiceToken},
			operatorcontext.HeaderName: {"not-a-token"}},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			handler, probe := newCommandHandler(t, discardLogger(), true)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, commandRequest(`{"name":"atlas"}`, headers))
			assertErrorEnvelope(t, recorder, http.StatusUnauthorized, "unauthorized", "/internal/things/thing-7")
			if probe.calls != 0 {
				t.Error("the handler ran for a rejected command")
			}
		})
	}
}

func TestInternalCommandRejectsAReplayedContext(t *testing.T) {
	handler, probe := newCommandHandler(t, discardLogger(), true)
	headers := map[string][]string{
		"Authorization":            {"Bearer " + testServiceToken},
		operatorcontext.HeaderName: {signOperatorContext(t, testSigningKey, "replay-1")},
	}
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, commandRequest(`{"name":"atlas"}`, headers))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, commandRequest(`{"name":"atlas"}`, headers))
	if first.Code != http.StatusOK {
		t.Fatalf("first use: %d", first.Code)
	}
	assertErrorEnvelope(t, second, http.StatusUnauthorized, "unauthorized", "/internal/things/thing-7")
	if probe.calls != 1 {
		t.Errorf("handler calls = %d, want 1", probe.calls)
	}
}

func TestInternalCommandWithoutVerifierRejectsEverything(t *testing.T) {
	handler, probe := newCommandHandler(t, discardLogger(), false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, commandRequest(`{"name":"atlas"}`, map[string][]string{
		"Authorization":            {"Bearer " + testServiceToken},
		operatorcontext.HeaderName: {signOperatorContext(t, testSigningKey, "no-verifier")},
	}))
	assertErrorEnvelope(t, recorder, http.StatusUnauthorized, "unauthorized", "/internal/things/thing-7")
	if probe.calls != 0 {
		t.Error("the handler ran without a verifier")
	}
}

func TestInternalCommandRejectsMalformedBodies(t *testing.T) {
	bodies := map[string]string{
		"unknown field":     `{"name":"atlas","organizationId":"x"}`,
		"oversized":         `{"name":"` + strings.Repeat("a", 100) + `"}`,
		"trailing document": `{"name":"atlas"} {}`,
		"not json":          `name=atlas`,
		"invalid utf-8":     "{\"name\":\"\xff\"}",
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			handler, _ := newCommandHandler(t, discardLogger(), true)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, commandRequest(body, map[string][]string{
				"Authorization":            {"Bearer " + testServiceToken},
				operatorcontext.HeaderName: {signOperatorContext(t, testSigningKey, "body-"+name)},
			}))
			assertErrorEnvelope(t, recorder, http.StatusBadRequest, "bad_request", "/internal/things/thing-7")
		})
	}
	t.Run("wrong content type", func(t *testing.T) {
		handler, _ := newCommandHandler(t, discardLogger(), true)
		request := commandRequest(`{"name":"atlas"}`, map[string][]string{
			"Authorization":            {"Bearer " + testServiceToken},
			operatorcontext.HeaderName: {signOperatorContext(t, testSigningKey, "body-content-type")},
		})
		request.Header.Set("Content-Type", "text/plain")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		assertErrorEnvelope(t, recorder, http.StatusBadRequest, "bad_request", "/internal/things/thing-7")
	})
}

func TestInternalCommandNeverLogsCredentials(t *testing.T) {
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler, _ := newCommandHandler(t, logger, true)
	token := signOperatorContext(t, testSigningKey, "logging-1")
	for _, headers := range []map[string][]string{
		{"Authorization": {"Bearer " + testServiceToken}, operatorcontext.HeaderName: {token}},
		{"Authorization": {"Bearer " + testServiceToken}, operatorcontext.HeaderName: {token + "x"}},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, commandRequest(`{"name":"atlas"}`, headers))
		for _, secret := range []string{testServiceToken, testSigningKey, token} {
			if strings.Contains(recorder.Body.String(), secret) {
				t.Error("a response contains a credential")
			}
		}
	}
	for _, secret := range []string{testServiceToken, testSigningKey, token} {
		if strings.Contains(logOutput.String(), secret) {
			t.Error("the log contains a credential")
		}
	}
}
