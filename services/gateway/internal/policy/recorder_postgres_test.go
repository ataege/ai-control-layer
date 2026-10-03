package policy

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

// seedRun inserts a passport and its run for an isolated organization. Passports are immutable
// (no UPDATE or DELETE), so the rows stay in the test database; their organization is random.
func seedRun(t *testing.T, pool *pgxpool.Pool) RunIdentity {
	t.Helper()
	ctx := context.Background()
	run := RunIdentity{OrganizationID: testdb.ID(t), RunID: testdb.ID(t)}
	passportID := testdb.ID(t)
	if _, err := pool.Exec(ctx,
		`INSERT INTO runtime.passports (id, organization_id, actor_id, task_version, admission_catalog_revision_id, scope, limits, expires_at)
		 VALUES ($1, $2, $3, 'reconcile_atlas_v1', 1, '{}', '{}', now() + interval '15 minutes')`,
		passportID, run.OrganizationID, testdb.ID(t)); err != nil {
		t.Fatalf("seed passport: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
		run.RunID, run.OrganizationID, passportID); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	return run
}

func storedReadAction(t *testing.T, run RunIdentity, stepNumber int) StoredAction {
	t.Helper()
	arguments, err := DecodeArguments(ToolReadInvoice, []byte(`{"invoice_id":"invoice_A01"}`))
	if err != nil {
		t.Fatal(err)
	}
	canonicalArguments, _ := CanonicalArguments(arguments)
	action := StoredAction{
		OrganizationID: run.OrganizationID, RunID: run.RunID, ActionID: testdb.ID(t), StepNumber: stepNumber,
		IdempotencyKey: testdb.ID(t), Tool: ToolReadInvoice, CanonicalArguments: canonicalArguments,
		CanonicalizationVersion: CanonicalizationVersion, EvaluatedRevisionID: 1,
	}
	action.ActionDigest, err = CanonicalAction{
		ActionID: action.ActionID, RunID: run.RunID, Arguments: arguments,
		PassportID: testdb.ID(t), PolicyRevisionID: 1,
	}.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return action
}

func TestRecorderStoresTheActionBeforeItsDecision(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	recorder := NewPostgresRecorder(pool)
	run := seedRun(t, pool)
	action := storedReadAction(t, run, 1)

	if err := recorder.StoreAction(ctx, action); err != nil {
		t.Fatalf("store action: %v", err)
	}
	var status string
	var storedArguments string
	if err := pool.QueryRow(ctx, `SELECT status, canonical_arguments::text FROM runtime.actions WHERE id = $1`, action.ActionID).
		Scan(&status, &storedArguments); err != nil || status != actionStatusProposed {
		t.Fatalf("stored action status = %q, err %v; want proposed before any decision", status, err)
	}

	decision := Decision{Outcome: OutcomeAllow, ActionID: action.ActionID, ActionStored: true, ActionDigest: action.ActionDigest, EvaluatedRevisionID: 1}
	if err := recorder.RecordDecision(ctx, run, decision); err != nil {
		t.Fatalf("record decision: %v", err)
	}
	var eventType, eventDecision, summary string
	var actionCreated, eventOccurred time.Time
	if err := pool.QueryRow(ctx,
		`SELECT a.status, a.created_at, e.event_type, e.decision, e.masked_summary::text, e.occurred_at
		   FROM runtime.actions a JOIN runtime.audit_events e ON e.action_id = a.id
		  WHERE a.id = $1`, action.ActionID).Scan(&status, &actionCreated, &eventType, &eventDecision, &summary, &eventOccurred); err != nil {
		t.Fatalf("read decision: %v", err)
	}
	if status != actionStatusAllowed || eventType != "action.allowed" || eventDecision != "allow" {
		t.Fatalf("status %q, event %q/%q; want allowed, action.allowed/allow", status, eventType, eventDecision)
	}
	if eventOccurred.Before(actionCreated) {
		t.Fatal("the decision event is older than its action")
	}
	if strings.Contains(summary, "invoice_A01") || !strings.Contains(summary, `"effect": "none"`) {
		t.Fatalf("masked summary = %s; want references only and no effect", summary)
	}

	// A second decision for the same action is refused: the action is decided exactly once.
	if err := recorder.RecordDecision(ctx, run, decision); err == nil {
		t.Fatal("a second decision for the same action was accepted")
	}
}

func TestRecorderAcceptsTheSameActionTwiceButNotAConflict(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	recorder := NewPostgresRecorder(pool)
	run := seedRun(t, pool)
	action := storedReadAction(t, run, 1)
	if err := recorder.StoreAction(ctx, action); err != nil {
		t.Fatal(err)
	}
	if err := recorder.StoreAction(ctx, action); err != nil {
		t.Fatalf("storing the identical action again: %v", err)
	}
	changed := action
	changed.ActionDigest[0] ^= 0xff
	if err := recorder.StoreAction(ctx, changed); err == nil {
		t.Fatal("an action with the same id and a different digest was accepted")
	}
	sameStep := storedReadAction(t, run, 1)
	if err := recorder.StoreAction(ctx, sameStep); err == nil {
		t.Fatal("a second action for the same step was accepted")
	}
}

func TestRecorderWritesADenialWithoutAnActionRow(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	recorder := NewPostgresRecorder(pool)
	run := seedRun(t, pool)
	decision := Decision{Outcome: OutcomeDeny, ReasonCode: ReasonInvalidArguments, ActionID: testdb.ID(t)}
	if err := recorder.RecordDecision(ctx, run, decision); err != nil {
		t.Fatalf("record denial: %v", err)
	}
	var reason string
	var actionID *string
	if err := pool.QueryRow(ctx,
		`SELECT reason_code, action_id::text FROM runtime.audit_events WHERE run_id = $1 AND event_type = 'action.denied'`,
		run.RunID).Scan(&reason, &actionID); err != nil {
		t.Fatal(err)
	}
	if reason != string(ReasonInvalidArguments) || actionID != nil {
		t.Fatalf("reason %q, action %v; want invalid_arguments and no action reference", reason, actionID)
	}
}

func TestRecorderWritesAnExportDenialEvent(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	recorder := NewPostgresRecorder(pool)
	run := seedRun(t, pool)
	action := storedReadAction(t, run, 1)
	if err := recorder.StoreAction(ctx, action); err != nil {
		t.Fatal(err)
	}
	decision := Decision{Outcome: OutcomeDeny, ReasonCode: ReasonReportExportRestricted, ActionID: action.ActionID,
		ActionStored: true, ActionDigest: action.ActionDigest, EvaluatedRevisionID: 1, AlternativeTemplate: TemplateVendorReconciliation}
	if err := recorder.RecordDecision(ctx, run, decision); err != nil {
		t.Fatal(err)
	}
	var eventType, summary string
	if err := pool.QueryRow(ctx, `SELECT event_type, masked_summary::text FROM runtime.audit_events WHERE action_id = $1`,
		action.ActionID).Scan(&eventType, &summary); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		LineageCheck        string `json:"lineageCheck"`
		AlternativeTemplate string `json:"alternativeTemplate"`
	}
	if err := json.Unmarshal([]byte(summary), &decoded); err != nil {
		t.Fatal(err)
	}
	if eventType != "report.export_denied" || decoded.LineageCheck != "failed" || decoded.AlternativeTemplate != TemplateVendorReconciliation {
		t.Fatalf("event %q with summary %s; want report.export_denied, lineage failed and the alternative", eventType, summary)
	}
}

func TestPassportScopeReaderUsesTheStoredPassport(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	run := RunIdentity{OrganizationID: testdb.ID(t), RunID: testdb.ID(t)}
	passportID := testdb.ID(t)
	scope, _ := json.Marshal(contracts.PassportScope{
		Tools: []contracts.ToolName{contracts.ToolReadInvoice, contracts.ToolQueueReport}, InvoiceIDs: []string{"invoice_A01"},
		VendorIDs: []string{"vendor_atlas"}, ReportTemplates: []contracts.ReportTemplate{contracts.TemplateVendorReconciliation},
		ProjectionRules: []string{"vendor_invoice_fields_v1"}, RecipientReferences: []string{"recipient:" + run.RunID + ":vendor_atlas"},
		AllowedModels: []string{"qwen3.5:4b"}, ApprovalRequiredTools: []contracts.ToolName{contracts.ToolQueueReport},
	})
	limits, _ := json.Marshal(contracts.PassportLimits{CallsTotal: 24, CallsAgent: 12, CallsSecurity: 12, TokensTotal: 20000,
		RequestTimeoutSeconds: 20, LocalMaxConcurrency: 2, ToolAttempts: 12, Corrections: 2, RunExpiryMinutes: 15})
	if _, err := pool.Exec(ctx, `INSERT INTO runtime.passports (id, organization_id, actor_id, task_version,
		admission_catalog_revision_id, scope, limits, expires_at)
		VALUES ($1, $2, $3, 'reconcile_atlas_v1', 1, $4, $5, now() + interval '15 minutes')`,
		passportID, run.OrganizationID, testdb.ID(t), scope, limits); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
		run.RunID, run.OrganizationID, passportID); err != nil {
		t.Fatal(err)
	}
	reader := NewPassportScopeReader(pool)
	loaded, err := reader.LoadScope(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PassportID != passportID || loaded.ToolAttemptLimit != 12 || len(loaded.AllowedTools) != 2 ||
		loaded.AllowedTemplates[0] != TemplateVendorReconciliation || loaded.ApprovalRequiredTools[0] != ToolQueueReport {
		t.Fatalf("loaded scope = %+v", loaded)
	}
	// The passport of another organization's view of the same run is not found.
	if _, err := reader.LoadScope(ctx, RunIdentity{OrganizationID: testdb.ID(t), RunID: run.RunID}); err == nil {
		t.Fatal("a scope was loaded for another organization")
	}
	// A stored passport that does not fit its contract (an unknown key) is never used.
	brokenRun := RunIdentity{OrganizationID: testdb.ID(t), RunID: testdb.ID(t)}
	brokenPassport := testdb.ID(t)
	if _, err := pool.Exec(ctx, `INSERT INTO runtime.passports (id, organization_id, actor_id, task_version,
		admission_catalog_revision_id, scope, limits, expires_at)
		VALUES ($1, $2, $3, 'reconcile_atlas_v1', 1, '{"allowEverything": true}', $4, now() + interval '15 minutes')`,
		brokenPassport, brokenRun.OrganizationID, testdb.ID(t), limits); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
		brokenRun.RunID, brokenRun.OrganizationID, brokenPassport); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.LoadScope(ctx, brokenRun); err == nil {
		t.Fatal("a passport with an unknown scope key was used")
	}
}

func TestCorrectionCounterCountsEveryDenialOfTheRun(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	recorder := NewPostgresRecorder(pool)
	counter := NewCorrectionCounter(pool)
	run := seedRun(t, pool)
	otherRun := seedRun(t, pool)

	// An allowed action, a denial with an action row, a denial without one (malformed arguments)
	// and an export denial; plus a denial of another run that must not count.
	allowed := storedReadAction(t, run, 1)
	denied := storedReadAction(t, run, 2)
	exported := storedReadAction(t, run, 3)
	for _, action := range []StoredAction{allowed, denied, exported} {
		if err := recorder.StoreAction(ctx, action); err != nil {
			t.Fatal(err)
		}
	}
	decisions := []Decision{
		{Outcome: OutcomeAllow, ActionID: allowed.ActionID, ActionStored: true, EvaluatedRevisionID: 1},
		{Outcome: OutcomeDeny, ReasonCode: ReasonResourceOutOfScope, ActionID: denied.ActionID, ActionStored: true, EvaluatedRevisionID: 1},
		{Outcome: OutcomeDeny, ReasonCode: ReasonInvalidArguments, ActionID: testdb.ID(t)},
		{Outcome: OutcomeDeny, ReasonCode: ReasonReportExportRestricted, ActionID: exported.ActionID, ActionStored: true, EvaluatedRevisionID: 1},
	}
	for _, decision := range decisions {
		if err := recorder.RecordDecision(ctx, run, decision); err != nil {
			t.Fatal(err)
		}
	}
	if err := recorder.RecordDecision(ctx, otherRun, Decision{Outcome: OutcomeDeny, ReasonCode: ReasonInvalidArguments, ActionID: testdb.ID(t)}); err != nil {
		t.Fatal(err)
	}

	used, err := counter.CorrectionsUsed(ctx, run)
	if err != nil || used != 3 {
		t.Fatalf("corrections used = %d, err %v; want 3", used, err)
	}
	if verdict := CheckCorrections(used, 2); verdict.Continue {
		t.Fatal("a third denial with a limit of 2 did not stop the run")
	}
	// A new counter (as after a worker restart) reads the same durable count.
	if again, _ := NewCorrectionCounter(pool).CorrectionsUsed(ctx, run); again != 3 {
		t.Fatalf("count after restart = %d, want 3", again)
	}
}
