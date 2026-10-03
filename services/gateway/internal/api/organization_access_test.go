package api

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/admission"
	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/testdb"
)

// crossOrganizationWorld is organization A's committed run with a vendor, invoices, an action
// awaiting approval and an internal report, read and commanded by organization B's operator.
// The pool-bound handlers (approvals) need committed rows, so the world removes them at the end.
type crossOrganizationWorld struct {
	pool                     *pgxpool.Pool
	owner, intruder          contracts.OperatorContext
	runID, passportID        string
	actionID, reportActionID string
	reportID                 string
	vendorID                 string
	invoiceIDs               []string
	handler                  http.Handler
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, arguments ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, arguments...); err != nil {
		t.Fatalf("fixture statement failed: %v", err)
	}
}

func openCrossOrganizationWorld(t *testing.T) *crossOrganizationWorld {
	t.Helper()
	pool := testdb.Open(t)
	suffix := testdb.ID(t)
	world := &crossOrganizationWorld{
		pool:     pool,
		owner:    contracts.OperatorContext{UserID: testdb.ID(t), OrganizationID: testdb.ID(t), Roles: []string{"operator", "reviewer"}},
		intruder: contracts.OperatorContext{UserID: testdb.ID(t), OrganizationID: testdb.ID(t), Roles: []string{"operator", "reviewer"}},
		runID:    testdb.ID(t), passportID: testdb.ID(t), actionID: testdb.ID(t), reportActionID: testdb.ID(t),
		reportID: testdb.ID(t), vendorID: "vendor_access_" + suffix,
		invoiceIDs: []string{"invoice_access_a01_" + suffix, "invoice_access_a02_" + suffix},
	}
	t.Cleanup(func() { world.remove(t) })

	organizationID := world.owner.OrganizationID
	mustExec(t, pool, `INSERT INTO demo.vendors (id, organization_id, name, registered_reporting_address)
		VALUES ($1, $2, 'Atlas', 'reports@atlas.example.com')`, world.vendorID, organizationID)
	for _, invoiceID := range world.invoiceIDs {
		mustExec(t, pool, `INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency,
			total_minor_units, issued_on, due_on) VALUES ($1, $2, $3, 'INV104', 'EUR', 125000, '2026-09-01', '2026-10-31')`,
			invoiceID, organizationID, world.vendorID)
	}
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	passport := contracts.Passport{
		PassportID: world.passportID, RunID: world.runID, OrganizationID: organizationID, ActorID: world.owner.UserID,
		TaskVersion: admission.TaskTemplateReconcileAtlas, AdmissionCatalogRevisionID: 1,
		IssuedAt: issuedAt, ExpiresAt: issuedAt.Add(15 * time.Minute),
		Scope: contracts.PassportScope{Tools: contracts.ToolNames, InvoiceIDs: world.invoiceIDs, VendorIDs: []string{world.vendorID},
			ReportTemplates: contracts.ReportTemplates, ProjectionRules: []string{"vendor_invoice_fields_v1"},
			RecipientReferences: []string{"recipient:" + world.runID + ":" + world.vendorID}, AllowedModels: []string{"qwen3.5:4b"},
			InternalNoteReadable: true, ApprovalRequiredTools: []contracts.ToolName{contracts.ToolQueueReport}},
		Limits: contracts.PassportLimits{CallsTotal: 24, CallsAgent: 12, CallsSecurity: 12, TokensTotal: 20000,
			RequestTimeoutSeconds: 20, LocalMaxConcurrency: 2, ToolAttempts: 12, Corrections: 2, RunExpiryMinutes: 15},
	}
	runtimeRepository := repository.New(pool)
	if err := runtimeRepository.InTransaction(context.Background(), func(tx repository.Tx) error {
		return tx.InsertAdmission(context.Background(), passport, repository.NewJob{ID: testdb.ID(t), Kind: contracts.JobKindAgentStep})
	}); err != nil {
		t.Fatalf("admission fixture: %v", err)
	}
	mustExec(t, pool, `UPDATE runtime.runs SET status = 'awaiting_approval' WHERE id = $1`, world.runID)
	digest := sha256.Sum256([]byte(world.actionID))
	insertAction := `INSERT INTO runtime.actions (id, organization_id, run_id, step_number, tool, canonical_arguments,
		canonicalization_version, action_digest, idempotency_key, evaluated_catalog_revision_id, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $8, 1, $9, now() + interval '10 minutes')`
	mustExec(t, pool, insertAction, world.reportActionID, organizationID, world.runID, 1, "create_report",
		`{"template":"internal_investigation_v1","source_invoice_ids":["`+world.invoiceIDs[0]+`"]}`, digest[:], world.reportActionID, "succeeded")
	reportDigest := sha256.Sum256([]byte(world.reportActionID))
	mustExec(t, pool, insertAction, world.actionID, organizationID, world.runID, 2, "queue_report",
		`{"report_id":"`+world.reportID+`","recipient_reference":"recipient:`+world.runID+`:`+world.vendorID+`"}`,
		reportDigest[:], world.actionID, "awaiting_approval")
	content := "Internal investigation of " + world.invoiceIDs[0]
	mustExec(t, pool, `INSERT INTO demo.reports (id, organization_id, run_id, created_by_action_id, template, classification,
		destination_class, title, content, content_hash) VALUES ($1, $2, $3, $4, 'internal_investigation_v1', 'internal_only',
		'internal_reviewers', 'Investigation', $5, sha256(convert_to($5, 'UTF8')))`,
		world.reportID, organizationID, world.runID, world.reportActionID, content)

	world.handler = newHandler(t, Dependencies{
		Admitter:  admission.New(runtimeRepository, catalog.NewLoader()),
		Canceller: runtimeRepository,
		Approvals: policy.NewApprovals(pool),
		Runs:      runtimeRepository,
		Database:  pool,
	})
	return world
}

// remove deletes the committed fixture. Passports reject DELETE by trigger, so the passport goes
// with triggers disabled for one transaction, which needs a superuser test database.
func (world *crossOrganizationWorld) remove(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	statements := []struct {
		sql      string
		argument string
	}{
		{"DELETE FROM runtime.audit_events WHERE run_id = $1", world.runID},
		{"DELETE FROM runtime.audit_events WHERE organization_id = $1", world.intruder.OrganizationID},
		{"DELETE FROM demo.reports WHERE run_id = $1", world.runID},
		{"DELETE FROM runtime.actions WHERE run_id = $1", world.runID},
		{"DELETE FROM runtime.jobs WHERE run_id = $1", world.runID},
		{"DELETE FROM runtime.runs WHERE id = $1", world.runID},
		{"DELETE FROM demo.invoices WHERE vendor_id = $1", world.vendorID},
		{"DELETE FROM demo.vendors WHERE id = $1", world.vendorID},
	}
	for _, statement := range statements {
		if _, err := world.pool.Exec(ctx, statement.sql, statement.argument); err != nil {
			t.Errorf("cleanup failed: %v", err)
			return
		}
	}
	transaction, err := world.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err = transaction.Exec(ctx, "SET LOCAL session_replication_role = replica"); err == nil {
		_, err = transaction.Exec(ctx, "DELETE FROM runtime.passports WHERE id = $1", world.passportID)
	}
	if err == nil {
		err = transaction.Commit(ctx)
	}
	if err != nil {
		t.Logf("passport fixture %s left in place (removing it needs a superuser)", world.passportID)
	}
}

// fingerprint hashes every row organization A owns in the runtime and demo tables the internal
// commands could touch, so "no runtime mutation" is checked row for row.
func (world *crossOrganizationWorld) fingerprint(t *testing.T) string {
	t.Helper()
	var combined strings.Builder
	for _, table := range []string{"runtime.passports", "runtime.runs", "runtime.jobs", "runtime.actions",
		"runtime.approvals", "runtime.audit_events", "runtime.review_payloads", "demo.reports",
		"demo.outbox_messages", "demo.invoices", "demo.vendors"} {
		var digest string
		err := world.pool.QueryRow(context.Background(), `SELECT coalesce(md5(string_agg(row_text, '|' ORDER BY row_text)), 'empty')
			FROM (SELECT t::text AS row_text FROM `+table+` AS t WHERE organization_id = $1) AS rows`,
			world.owner.OrganizationID).Scan(&digest)
		if err != nil {
			t.Fatalf("fingerprint %s: %v", table, err)
		}
		combined.WriteString(table + "=" + digest + ";")
	}
	return combined.String()
}

func (world *crossOrganizationWorld) call(t *testing.T, operator contracts.OperatorContext, method, path, body, tokenID string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Authorization", "Bearer "+testServiceToken)
	request.Header.Set("X-Operator-Context", signOperatorContext(t, operator, tokenID))
	recorder := httptest.NewRecorder()
	world.handler.ServeHTTP(recorder, request)
	return recorder
}

// GO-57: every internal command and read, called with a valid service identity and organization
// B's operator context against organization A's run, action and resources, is rejected and leaves
// A's rows identical; organization A's own operator still reaches the same records.
func TestPostgresAnotherOrganizationCannotReachTheRunOrItsResources(t *testing.T) {
	world := openCrossOrganizationWorld(t)
	before := world.fingerprint(t)

	startBody := `{"template":"reconcile_atlas_v1","vendorId":"` + world.vendorID + `","invoiceIds":["` +
		strings.Join(world.invoiceIDs, `","`) + `"],"destination":"` + world.vendorID + `"}`
	attempts := []struct {
		name, method, path, body string
		wantStatus               []int
	}{
		{"start a run on A's invoices", http.MethodPost, "/internal/runs", startBody, []int{http.StatusBadRequest, http.StatusServiceUnavailable}},
		{"cancel A's run", http.MethodPost, "/internal/runs/" + world.runID + "/cancel", "", []int{http.StatusNotFound}},
		{"read A's report", http.MethodGet, "/internal/runs/" + world.runID + "/reports/" + world.reportID, "", []int{http.StatusNotFound}},
		{"approve A's action", http.MethodPost, "/internal/actions/" + world.actionID + "/approval", `{"decision":"approve"}`,
			[]int{http.StatusNotFound, http.StatusForbidden}},
		{"reject A's action", http.MethodPost, "/internal/actions/" + world.actionID + "/approval", `{"decision":"reject"}`,
			[]int{http.StatusNotFound, http.StatusForbidden}},
		{"read A's review", http.MethodGet, "/internal/actions/" + world.actionID + "/review", "", []int{http.StatusNotFound, http.StatusForbidden}},
		{"read A's run state", http.MethodGet, "/internal/runs/" + world.runID, "", []int{http.StatusNotFound}},
		{"read A's run events", http.MethodGet, "/internal/runs/" + world.runID + "/events", "", []int{http.StatusNotFound}},
		{"read A's run usage", http.MethodGet, "/internal/runs/" + world.runID + "/usage", "", []int{http.StatusNotFound}},
	}
	for index, attempt := range attempts {
		recorder := world.call(t, world.intruder, attempt.method, attempt.path, attempt.body, "intruder-"+string(rune('a'+index)))
		accepted := false
		for _, status := range attempt.wantStatus {
			accepted = accepted || recorder.Code == status
		}
		if !accepted {
			t.Errorf("%s: status %d (%s), want one of %v", attempt.name, recorder.Code, recorder.Body.String(), attempt.wantStatus)
		}
		if strings.Contains(recorder.Body.String(), "Internal investigation") || strings.Contains(recorder.Body.String(), world.vendorID) {
			t.Errorf("%s: the response discloses organization A's data: %s", attempt.name, recorder.Body.String())
		}
	}
	// The organization-wide security reads answer B with B's own (empty) records, never A's.
	for index, path := range []string{"/internal/security/summary", "/internal/security/assessments", "/internal/security/events"} {
		recorder := world.call(t, world.intruder, http.MethodGet, path, "", "intruder-security-"+string(rune('a'+index)))
		if recorder.Code != http.StatusOK {
			t.Errorf("%s: status %d (%s)", path, recorder.Code, recorder.Body.String())
		}
		for _, secret := range []string{world.runID, world.actionID, world.reportID, world.owner.OrganizationID} {
			if strings.Contains(recorder.Body.String(), secret) {
				t.Errorf("%s discloses organization A's record %s", path, secret)
			}
		}
	}
	if after := world.fingerprint(t); after != before {
		t.Errorf("organization A's rows changed\nbefore %s\nafter  %s", before, after)
	}
	var intruderPassports int
	if err := world.pool.QueryRow(context.Background(), `SELECT count(*) FROM runtime.passports WHERE organization_id = $1`,
		world.intruder.OrganizationID).Scan(&intruderPassports); err != nil || intruderPassports != 0 {
		t.Errorf("organization B got %d passports (err %v)", intruderPassports, err)
	}

	// Positive controls: A's own operator reaches the same records.
	for _, path := range []string{"/internal/runs/" + world.runID, "/internal/runs/" + world.runID + "/events"} {
		if recorder := world.call(t, world.owner, http.MethodGet, path, "", "owner-read-"+path); recorder.Code != http.StatusOK {
			t.Errorf("owner read %s: %d %s", path, recorder.Code, recorder.Body.String())
		}
	}
	if recorder := world.call(t, world.owner, http.MethodGet, "/internal/runs/"+world.runID+"/reports/"+world.reportID, "", "owner-report"); recorder.Code != http.StatusOK {
		t.Errorf("owner report read: %d %s", recorder.Code, recorder.Body.String())
	}
	if recorder := world.call(t, world.owner, http.MethodPost, "/internal/runs/"+world.runID+"/cancel", "", "owner-cancel"); recorder.Code != http.StatusOK {
		t.Errorf("owner cancel: %d %s", recorder.Code, recorder.Body.String())
	}
}
