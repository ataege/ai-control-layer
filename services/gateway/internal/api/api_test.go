package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"starter/services/gateway/internal/admission"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/health"
	"starter/services/gateway/internal/httpserver"
	"starter/services/gateway/internal/logging"
	"starter/services/gateway/internal/operatorcontext"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/testdb"
)

const (
	testServiceToken = "api-test-service-token-0123456789abcdef"
	testSigningKey   = "api-test-signing-key-0123456789abcdefgh"
)

var testOperator = contracts.OperatorContext{
	UserID:         "2c3d4e5f-6a7b-4c8d-9e0f-1a2b3c4d5e6f",
	OrganizationID: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
	Roles:          []string{"operator"},
}

// fakeAdmitter is a labelled test double for admission; it records what it was given.
type fakeAdmitter struct {
	passport contracts.Passport
	err      error
	calls    int
	operator contracts.OperatorContext
	request  contracts.StartRunRequest
}

func (admitter *fakeAdmitter) Admit(_ context.Context, operator contracts.OperatorContext, request contracts.StartRunRequest) (contracts.Passport, error) {
	admitter.calls++
	admitter.operator, admitter.request = operator, request
	return admitter.passport, admitter.err
}

// signOperatorContext signs an X-Operator-Context token the way the API does.
func signOperatorContext(t *testing.T, operator contracts.OperatorContext, tokenID string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "HS256"})
	now := time.Now()
	claims, _ := json.Marshal(map[string]any{"ctx": operator, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix(),
		"aud": "gateway", "iss": "gateway-client", "jti": tokenID})
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(testSigningKey))
	mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func newHandler(t *testing.T, dependencies Dependencies) http.Handler {
	t.Helper()
	verifier, err := operatorcontext.NewVerifier(logging.NewSecret(testSigningKey))
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	logger := slog.New(slog.DiscardHandler)
	return httpserver.NewHandler(httpserver.Options{
		Logger:           logger,
		Health:           health.Handler{Logger: logger, DatabaseTimeout: time.Second},
		ServiceToken:     logging.NewSecret(testServiceToken),
		OperatorContext:  verifier,
		InternalCommands: Commands(dependencies),
	})
}

func startRun(t *testing.T, handler http.Handler, operator contracts.OperatorContext, tokenID, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/internal/runs", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+testServiceToken)
	request.Header.Set(operatorcontext.HeaderName, signOperatorContext(t, operator, tokenID))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeStrict(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := contracts.DecodeStrict(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("body %q does not match its contract: %v", recorder.Body.String(), err)
	}
}

func assertError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d (body %q)", recorder.Code, status, recorder.Body.String())
	}
	var envelope health.ErrorResponse
	decodeStrict(t, recorder, &envelope)
	if envelope.Error.Code != code || envelope.Error.Message == "" || envelope.StatusCode != status {
		t.Errorf("envelope = %+v, want code %q", envelope, code)
	}
}

const validBody = `{"template":"reconcile_atlas_v1","invoiceIds":["invoice_A01"],"destination":"vendor_Atlas"}`

func TestStartRunReturnsTheAdmittedIdsAndPassesOnlyTheVerifiedOperator(t *testing.T) {
	admitter := &fakeAdmitter{passport: contracts.Passport{RunID: "run-1", PassportID: "passport-1"}}
	recorder := startRun(t, newHandler(t, Dependencies{Admitter: admitter}), testOperator, "ok-1", validBody)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %q", recorder.Code, recorder.Body.String())
	}
	var response contracts.StartRunResponse
	decodeStrict(t, recorder, &response)
	if response.RunID != "run-1" || response.PassportID != "passport-1" {
		t.Errorf("response = %+v", response)
	}
	if admitter.operator.OrganizationID != testOperator.OrganizationID || admitter.operator.UserID != testOperator.UserID ||
		admitter.request.Template != "reconcile_atlas_v1" || admitter.request.Destination != "vendor_Atlas" {
		t.Errorf("admitter got operator %+v request %+v", admitter.operator, admitter.request)
	}
}

func TestStartRunMapsRejectionAndUnavailability(t *testing.T) {
	rejected := &fakeAdmitter{err: &admission.Rejection{Code: contracts.ReasonResourceOutOfScope, Message: "invoice \"x\" is not available"}}
	assertError(t, startRun(t, newHandler(t, Dependencies{Admitter: rejected}), testOperator, "reject-1", validBody),
		http.StatusBadRequest, "resource_out_of_scope")

	unavailable := &fakeAdmitter{err: admission.ErrUnavailable}
	assertError(t, startRun(t, newHandler(t, Dependencies{Admitter: unavailable}), testOperator, "unavailable-1", validBody),
		http.StatusServiceUnavailable, "decision_unavailable")

	unexpected := &fakeAdmitter{err: errors.New("unexpected")}
	assertError(t, startRun(t, newHandler(t, Dependencies{Admitter: unexpected}), testOperator, "unexpected-1", validBody),
		http.StatusServiceUnavailable, "decision_unavailable")
}

func TestStartRunRejectsBadBodiesBeforeAdmission(t *testing.T) {
	bodies := map[string]string{
		"identity in the body": `{"template":"reconcile_atlas_v1","invoiceIds":["a"],"destination":"v","organizationId":"x"}`,
		"oversized":            `{"template":"reconcile_atlas_v1","invoiceIds":["` + strings.Repeat("a", 70<<10) + `"],"destination":"v"}`,
		"not an object":        `["reconcile_atlas_v1"]`,
		"two documents":        validBody + validBody,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			admitter := &fakeAdmitter{}
			assertError(t, startRun(t, newHandler(t, Dependencies{Admitter: admitter}), testOperator, "body-"+name, body),
				http.StatusBadRequest, "bad_request")
			if admitter.calls != 0 {
				t.Error("admission ran for a malformed body")
			}
		})
	}
}

func TestStartRunHandlerRefusesWithoutAVerifiedOperator(t *testing.T) {
	admitter := &fakeAdmitter{}
	recorder := httptest.NewRecorder()
	StartRunHandler(admitter).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/internal/runs", strings.NewReader(validBody)))
	assertError(t, recorder, http.StatusUnauthorized, "unauthorized")
	if admitter.calls != 0 {
		t.Error("admission ran without a verified operator")
	}
}

func TestStoredReportViewerComesFromTheVerifiedOperatorOnly(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/internal/runs/x/reports/y", nil)
	if _, verified := StoredReportViewer(request); verified {
		t.Error("a viewer was derived without a verified operator")
	}
	viewer, verified := StoredReportViewer(request.WithContext(operatorcontext.WithOperator(request.Context(), testOperator)))
	if !verified || viewer.OrganizationID != testOperator.OrganizationID || !viewer.MayReadInternal {
		t.Errorf("viewer = %+v, verified = %v", viewer, verified)
	}
}

// End to end through the real route guard, admission and PostgreSQL, inside one rolled-back
// outer transaction with its own catalog revision and demo records.
func TestPostgresStartRunAdmitsThroughTheRealBoundary(t *testing.T) {
	pool := testdb.Open(t)
	outer, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = outer.Rollback(context.Background()) })
	operator := contracts.OperatorContext{UserID: testdb.ID(t), OrganizationID: testdb.ID(t), Roles: []string{"operator"}}
	suffix := testdb.ID(t)
	vendorID, invoiceID := "vendor_"+suffix, "invoice_"+suffix
	statements := []struct {
		sql       string
		arguments []any
	}{
		{`INSERT INTO demo.vendors (id, organization_id, name, registered_reporting_address) VALUES ($1, $2, 'Atlas', 'reports@atlas.example.com')`,
			[]any{vendorID, operator.OrganizationID}},
		{`INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency, total_minor_units, issued_on, due_on)
			VALUES ($1, $2, $3, 'INV104', 'EUR', 125000, '2026-09-01', '2026-10-31')`, []any{invoiceID, operator.OrganizationID, vendorID}},
	}
	for _, statement := range statements {
		if _, err := outer.Exec(context.Background(), statement.sql, statement.arguments...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	var revisionID int64
	err = outer.QueryRow(context.Background(), `INSERT INTO app.control_catalog_revisions
		(schema_version, source_file_name, source_text, file_digest, content, import_source)
		VALUES (1, 'policy.yaml', 'api test', repeat('b', 64), $1, 'command') RETURNING id`,
		`{"schema_version":1,"allowed_models":["qwen3.5:4b"],"budgets":{"calls_total":24,"calls_agent":12,
		"calls_security":12,"tokens_total":20000,"request_timeout_seconds":20,"local_max_concurrency":2,
		"run_expiry_minutes":15,"tool_attempts":12,"corrections":2},
		"reports":{"enabled_templates":["internal_investigation_v1","vendor_reconciliation_v1"]}}`).Scan(&revisionID)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := outer.Exec(context.Background(), `INSERT INTO app.control_catalog_pointer (id, active_revision_id) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET active_revision_id = EXCLUDED.active_revision_id`, revisionID); err != nil {
		t.Fatalf("pointer: %v", err)
	}
	handler := newHandler(t, Dependencies{Admitter: admission.New(repository.New(outer)), Database: outer})

	body := `{"template":"reconcile_atlas_v1","vendorId":"` + vendorID + `","invoiceIds":["` + invoiceID + `"],"destination":"` + vendorID + `"}`
	recorder := startRun(t, handler, operator, "e2e-1", body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %q", recorder.Code, recorder.Body.String())
	}
	var response contracts.StartRunResponse
	decodeStrict(t, recorder, &response)
	passport, err := repository.New(outer).Passport(context.Background(), operator.OrganizationID, response.RunID)
	if err != nil || passport.PassportID != response.PassportID || passport.ActorID != operator.UserID {
		t.Fatalf("stored passport %+v err %v", passport, err)
	}

	// A request for an invoice the organization does not hold is rejected with its code and
	// leaves no second passport.
	rejectedBody := strings.Replace(body, `"invoiceIds":["`+invoiceID+`"]`, `"invoiceIds":["invoice_elsewhere"]`, 1)
	assertError(t, startRun(t, handler, operator, "e2e-2", rejectedBody), http.StatusBadRequest, "resource_out_of_scope")
	var passports int
	if err := outer.QueryRow(context.Background(), "SELECT count(*) FROM runtime.passports WHERE organization_id = $1",
		operator.OrganizationID).Scan(&passports); err != nil || passports != 1 {
		t.Errorf("passports = %d err %v, want 1", passports, err)
	}

	// The stored report route is mounted behind the same guard.
	reportRequest := httptest.NewRequest(http.MethodGet, "/internal/runs/"+response.RunID+"/reports/"+testdb.ID(t), nil)
	reportRecorder := httptest.NewRecorder()
	handler.ServeHTTP(reportRecorder, reportRequest)
	if reportRecorder.Code != http.StatusUnauthorized {
		t.Errorf("stored report without credentials: status %d", reportRecorder.Code)
	}
}
