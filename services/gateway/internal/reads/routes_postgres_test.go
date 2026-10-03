package reads_test

import (
	"context"
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

	"starter/services/gateway/internal/api"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/health"
	"starter/services/gateway/internal/httpserver"
	"starter/services/gateway/internal/logging"
	"starter/services/gateway/internal/operatorcontext"
	"starter/services/gateway/internal/reads"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/testdb"
)

const (
	routeServiceToken = "reads-route-test-service-token-0123456789"
	routeSigningKey   = "reads-route-test-signing-key-0123456789ab"
)

// signedContext is an X-Operator-Context token as NestJS signs it (HS256, audience gateway).
func signedContext(t *testing.T, operator contracts.OperatorContext, tokenID string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "HS256"})
	now := time.Now()
	claims, _ := json.Marshal(map[string]any{"ctx": operator, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix(),
		"aud": "gateway", "iss": "gateway-client", "jti": tokenID})
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(routeSigningKey))
	mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// TestPostgresReadRoutesThroughTheGatewayHandler calls the seven read routes through the production
// handler tree (httpserver.NewHandler with api.Commands): service token, signed operator context,
// the mounted handlers and the database, as NestJS reaches them (GO-24, GO-83).
func TestPostgresReadRoutesThroughTheGatewayHandler(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	owner := contracts.OperatorContext{UserID: testdb.ID(t), OrganizationID: testdb.ID(t), Roles: []string{"operator"}}
	intruder := contracts.OperatorContext{UserID: testdb.ID(t), OrganizationID: testdb.ID(t), Roles: []string{"operator"}}

	// A committed run of the owner's organization with one event.
	runID, passportID := testdb.ID(t), testdb.ID(t)
	runtimeRepository := repository.New(pool)
	err := runtimeRepository.InTransaction(ctx, func(tx repository.Tx) error {
		if _, err := tx.Raw().Exec(ctx, `INSERT INTO runtime.passports (id, organization_id, actor_id, task_version,
			admission_catalog_revision_id, scope, limits, expires_at)
			VALUES ($1, $2, $3, 'reconcile_atlas_v1', 1, '{}', '{}', now() + interval '15 minutes')`,
			passportID, owner.OrganizationID, owner.UserID); err != nil {
			return err
		}
		if _, err := tx.Raw().Exec(ctx, `INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
			runID, owner.OrganizationID, passportID); err != nil {
			return err
		}
		_, err := tx.AppendEvent(ctx, repository.NewEvent{OrganizationID: owner.OrganizationID, RunID: &runID, EventType: contracts.EventRunStarted})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	verifier, err := operatorcontext.NewVerifier(logging.NewSecret(routeSigningKey))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.DiscardHandler)
	handler := httpserver.NewHandler(httpserver.Options{
		Logger: logger, Health: health.Handler{Logger: logger, DatabaseTimeout: time.Second},
		ServiceToken: logging.NewSecret(routeServiceToken), OperatorContext: verifier,
		InternalCommands: api.Commands(api.Dependencies{Runs: runtimeRepository, Database: pool}),
	})
	call := func(operator contracts.OperatorContext, path string, withServiceToken bool) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if withServiceToken {
			request.Header.Set("Authorization", "Bearer "+routeServiceToken)
		}
		request.Header.Set(operatorcontext.HeaderName, signedContext(t, operator, "reads-route-"+testdb.ID(t)))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	runPaths := map[string]any{
		"/internal/runs/" + runID:               &contracts.RunState{},
		"/internal/runs/" + runID + "/events":   &reads.RunEventPage{},
		"/internal/runs/" + runID + "/usage":    &reads.RunUsage{},
		"/internal/runs/" + runID + "/passport": &contracts.Passport{},
	}
	for path, target := range runPaths {
		recorder := call(owner, path, true)
		if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), target) != nil {
			t.Fatalf("owner %s: %d %s", path, recorder.Code, recorder.Body.String())
		}
		// Another organization learns nothing, and no request passes without the service token.
		if recorder := call(intruder, path, true); recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), passportID) {
			t.Fatalf("intruder %s: %d %s", path, recorder.Code, recorder.Body.String())
		}
		if recorder := call(owner, path, false); recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s without the service token: %d", path, recorder.Code)
		}
	}
	if page := runPaths["/internal/runs/"+runID+"/events"].(*reads.RunEventPage); len(page.Events) != 1 || page.Events[0].EventType != contracts.EventRunStarted {
		t.Fatalf("run events %+v", page)
	}

	// The catalog status is global: every verified operator reads it, and it needs the service token.
	for _, operator := range []contracts.OperatorContext{owner, intruder} {
		var status reads.CatalogStatus
		recorder := call(operator, "/internal/catalog/active", true)
		if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &status) != nil || status.ActiveRevisionID == nil || len(status.Controls) != 3 {
			t.Fatalf("catalog status %d %s", recorder.Code, recorder.Body.String())
		}
	}
	if recorder := call(owner, "/internal/catalog/active", false); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("catalog status without the service token: %d", recorder.Code)
	}

	recorder := call(owner, "/internal/security/summary", true)
	var summary reads.SecuritySummary
	if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &summary) != nil || summary.OrganizationID != owner.OrganizationID {
		t.Fatalf("summary %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = call(owner, "/internal/security/assessments", true)
	var assessments reads.AssessmentPage
	if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &assessments) != nil {
		t.Fatalf("assessments %d %s", recorder.Code, recorder.Body.String())
	}
	// The organization-wide event page returns the committed event once its transaction is final.
	deadline := time.Now().Add(20 * time.Second)
	cursor := ""
	for {
		path := "/internal/security/events"
		if cursor != "" {
			path += "?cursor=" + cursor
		}
		recorder = call(owner, path, true)
		var page reads.SecurityEventPage
		if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &page) != nil {
			t.Fatalf("security events %d %s", recorder.Code, recorder.Body.String())
		}
		cursor = page.NextCursor
		if len(page.Events) == 1 && page.Events[0].RunID != nil && *page.Events[0].RunID == runID {
			break
		}
		if len(page.Events) > 1 || time.Now().After(deadline) {
			t.Fatalf("security events %s", recorder.Body.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, path := range []string{"/internal/security/summary", "/internal/security/assessments", "/internal/security/events"} {
		if recorder := call(intruder, path, true); recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), runID) {
			t.Fatalf("intruder %s: %d %s", path, recorder.Code, recorder.Body.String())
		}
	}
	t.Logf("evidence GO-24/GO-83: seven read routes through httpserver.NewHandler and api.Commands: owner 200 with strict X-11/X-12 and draft shapes, other organization 404 or its own empty records, no service token 401")
}
