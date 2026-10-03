package policy

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/provenance"
	"starter/services/gateway/internal/testdb"
)

// reviewWorld is an Atlas run with a contract-valid passport, a vendor with a registered address
// and a vendor report stored through provenance, ready for a queue_report review.
type reviewWorld struct {
	pool       *pgxpool.Pool
	run        RunIdentity
	scope      PassportScope
	invoiceID  string
	reportID   string
	reference  string
	address    string
	nextStep   int
	reportBody string
}

func openReviewWorld(t *testing.T) *reviewWorld {
	t.Helper()
	pool := testdb.Open(t)
	ctx := context.Background()
	suffix := testdb.ID(t)[:8]
	world := &reviewWorld{
		pool: pool, run: RunIdentity{OrganizationID: testdb.ID(t), RunID: testdb.ID(t)},
		invoiceID: "invoice_A01_" + suffix, address: "reports-" + suffix + "@atlas.example.com",
		reportBody: "Vendor reconciliation " + suffix,
	}
	vendorID := "vendor_Atlas_" + suffix
	world.reference = "recipient:" + world.run.RunID + ":" + vendorID
	passportID := testdb.ID(t)
	mustExec(t, pool, `INSERT INTO demo.vendors (id, organization_id, name, registered_reporting_address) VALUES ($1, $2, 'Atlas', $3)`,
		vendorID, world.run.OrganizationID, world.address)
	mustExec(t, pool, `INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency, total_minor_units, issued_on, due_on)
	                   VALUES ($1, $2, $3, 'INV104', 'EUR', 125000, '2026-09-01', '2026-10-31')`, world.invoiceID, world.run.OrganizationID, vendorID)
	scope, _ := json.Marshal(contracts.PassportScope{
		Tools: []contracts.ToolName{contracts.ToolCreateReport, contracts.ToolQueueReport}, InvoiceIDs: []string{world.invoiceID},
		VendorIDs: []string{vendorID}, ReportTemplates: []contracts.ReportTemplate{contracts.TemplateVendorReconciliation},
		ProjectionRules: []string{"vendor_invoice_fields_v1"}, RecipientReferences: []string{world.reference},
		AllowedModels: []string{"qwen3.5:4b"}, ApprovalRequiredTools: []contracts.ToolName{contracts.ToolQueueReport},
	})
	limits, _ := json.Marshal(contracts.PassportLimits{CallsTotal: 24, CallsAgent: 12, CallsSecurity: 12, TokensTotal: 20000,
		RequestTimeoutSeconds: 20, LocalMaxConcurrency: 2, ToolAttempts: 12, Corrections: 2, RunExpiryMinutes: 15})
	expiresAt := time.Now().Add(15 * time.Minute).UTC().Truncate(time.Microsecond)
	mustExec(t, pool, `INSERT INTO runtime.passports (id, organization_id, actor_id, task_version, admission_catalog_revision_id, scope, limits, expires_at)
	                   VALUES ($1, $2, $3, 'reconcile_atlas_v1', 1, $4, $5, $6)`, passportID, world.run.OrganizationID, testdb.ID(t), scope, limits, expiresAt)
	mustExec(t, pool, `INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
		world.run.RunID, world.run.OrganizationID, passportID)
	world.scope = PassportScope{
		OrganizationID: world.run.OrganizationID, RunID: world.run.RunID, PassportID: passportID,
		AllowedTools: []ToolName{ToolCreateReport, ToolQueueReport}, AllowedInvoiceIDs: []string{world.invoiceID},
		AllowedTemplates: []string{TemplateVendorReconciliation}, RecipientReferences: []string{world.reference},
		ApprovalRequiredTools: []ToolName{ToolQueueReport}, ToolAttemptLimit: 12, ExpiresAt: expiresAt,
	}

	// The report needs a creating action of this run.
	creatingAction := world.storeAction(t, ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":["`+world.invoiceID+`"]}`)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	stored, err := provenance.StoreReport(ctx, tx, provenance.NewReport{
		OrganizationID: world.run.OrganizationID, RunID: world.run.RunID, CreatedByActionID: creatingAction,
		Template: provenance.VendorReconciliationV1,
		Sources: []provenance.Source{{Kind: provenance.SourceInvoice, ID: world.invoiceID, Version: 1,
			Classification: provenance.VendorShareable, ConsumedFields: provenance.VendorInvoiceFieldsV1.Fields}},
		Title: "Atlas reconciliation", Content: world.reportBody,
	})
	if err != nil {
		t.Fatalf("store report: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	world.reportID = stored.ID
	return world
}

// storeAction stores a proposal of this run as an action and returns its id.
func (world *reviewWorld) storeAction(t *testing.T, tool ToolName, rawArguments string) string {
	t.Helper()
	world.nextStep++
	arguments, err := DecodeArguments(tool, []byte(rawArguments))
	if err != nil {
		t.Fatal(err)
	}
	canonicalArguments, _ := CanonicalArguments(arguments)
	action := StoredAction{
		OrganizationID: world.run.OrganizationID, RunID: world.run.RunID, ActionID: testdb.ID(t), StepNumber: world.nextStep,
		IdempotencyKey: testdb.ID(t), Tool: tool, CanonicalArguments: canonicalArguments,
		CanonicalizationVersion: CanonicalizationVersion, EvaluatedRevisionID: 1,
	}
	action.ActionDigest, _ = CanonicalAction{ActionID: action.ActionID, RunID: world.run.RunID, Arguments: arguments,
		PassportID: world.scope.PassportID, PolicyRevisionID: 1}.Digest()
	if err := NewPostgresRecorder(world.pool).StoreAction(context.Background(), action); err != nil {
		t.Fatal(err)
	}
	return action.ActionID
}

func (world *reviewWorld) queueAction(t *testing.T) StoredAction {
	t.Helper()
	raw := `{"report_id":"` + world.reportID + `","recipient_reference":"` + world.reference + `"}`
	actionID := world.storeAction(t, ToolQueueReport, raw)
	arguments, _ := DecodeArguments(ToolQueueReport, []byte(raw))
	canonicalArguments, _ := CanonicalArguments(arguments)
	return StoredAction{OrganizationID: world.run.OrganizationID, RunID: world.run.RunID, ActionID: actionID,
		Tool: ToolQueueReport, CanonicalArguments: canonicalArguments, EvaluatedRevisionID: 1}
}

func readFrozenPayload(t *testing.T, pool *pgxpool.Pool, payloadID string) (ReviewPayload, []byte) {
	t.Helper()
	var raw, digest []byte
	if err := pool.QueryRow(context.Background(), `SELECT payload, payload_digest FROM runtime.review_payloads WHERE id = $1`, payloadID).
		Scan(&raw, &digest); err != nil {
		t.Fatal(err)
	}
	var payload ReviewPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	return payload, digest
}

func TestFreezeHoldsTheExactReviewedMaterial(t *testing.T) {
	world := openReviewWorld(t)
	ctx := context.Background()
	action := world.queueAction(t)
	frozen, err := NewPostgresReviewFreezer(world.pool).Freeze(ctx, world.run, action, world.scope)
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	payload, storedDigest := readFrozenPayload(t, world.pool, frozen.PayloadID)

	if payload.ActionID != action.ActionID || payload.RunID != world.run.RunID || payload.PassportID != world.scope.PassportID ||
		payload.PolicyRevisionID != 1 || payload.Tool != ToolQueueReport || payload.CanonicalizationVersion != CanonicalizationVersion {
		t.Fatalf("identity fields: %+v", payload)
	}
	if string(payload.CanonicalArguments) == "" || payload.Recipient == nil || payload.Recipient.Address != world.address ||
		payload.Recipient.Reference != world.reference {
		t.Fatalf("arguments or recipient: %+v", payload.Recipient)
	}
	report := payload.Report
	if report == nil || report.ID != world.reportID || report.Content != world.reportBody || report.Classification != provenance.VendorShareable ||
		report.Template != TemplateVendorReconciliation || report.TemplateVersion != provenance.VendorReconciliationV1.Version ||
		report.ProjectionRule == nil || *report.ProjectionRule != provenance.VendorInvoiceFieldsV1.Name || report.SourceManifestDigest == "" {
		t.Fatalf("report: %+v", report)
	}
	if len(report.Sources) != 1 || report.Sources[0].ID != world.invoiceID || report.Sources[0].Version != 1 {
		t.Fatalf("sources: %+v", report.Sources)
	}
	if expires, _ := time.Parse(time.RFC3339Nano, payload.ExpiresAt); !expires.Equal(world.scope.ExpiresAt) {
		t.Fatalf("expires_at = %s, want the passport expiry %s", payload.ExpiresAt, world.scope.ExpiresAt)
	}
	// The stored digest is the digest of the stored payload (jsonb reorders keys; the typed
	// payload re-encodes canonically).
	recomputed, _, _ := payload.Digest()
	if string(recomputed[:]) != string(storedDigest) || recomputed != frozen.PayloadDigest {
		t.Fatal("the stored payload digest does not match its payload")
	}

	// A second freeze for the same action is refused; the frozen row cannot be changed.
	if _, err := NewPostgresReviewFreezer(world.pool).Freeze(ctx, world.run, action, world.scope); err == nil {
		t.Fatal("a second freeze for the same action was accepted")
	}
	if _, err := world.pool.Exec(ctx, `UPDATE runtime.review_payloads SET payload = '{}' WHERE id = $1`, frozen.PayloadID); err == nil {
		t.Fatal("a frozen payload was updated")
	}
}

func TestSourceChangeAfterFreezeIsDetected(t *testing.T) {
	world := openReviewWorld(t)
	ctx := context.Background()
	frozen, err := NewPostgresReviewFreezer(world.pool).Freeze(ctx, world.run, world.queueAction(t), world.scope)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := readFrozenPayload(t, world.pool, frozen.PayloadID)
	current := func() map[string]int {
		tx, err := world.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		versions, err := provenance.CurrentInvoiceVersions(ctx, tx, world.run.OrganizationID, []string{world.invoiceID})
		if err != nil {
			t.Fatal(err)
		}
		return versions
	}
	if stale := StaleSources(payload, current()); len(stale) != 0 {
		t.Fatalf("stale right after freezing: %v", stale)
	}
	mustExec(t, world.pool, `UPDATE demo.invoices SET version = version + 1 WHERE id = $1`, world.invoiceID)
	if stale := StaleSources(payload, current()); len(stale) != 1 || stale[0] != world.invoiceID {
		t.Fatalf("stale after a source change = %v, want the invoice", stale)
	}
}

func TestUnresolvableRecipientFreezesNothing(t *testing.T) {
	world := openReviewWorld(t)
	ctx := context.Background()
	raw := `{"report_id":"` + world.reportID + `","recipient_reference":"recipient:` + world.run.RunID + `:vendor_nobody"}`
	actionID := world.storeAction(t, ToolQueueReport, raw)
	arguments, _ := DecodeArguments(ToolQueueReport, []byte(raw))
	canonicalArguments, _ := CanonicalArguments(arguments)
	action := StoredAction{OrganizationID: world.run.OrganizationID, RunID: world.run.RunID, ActionID: actionID,
		Tool: ToolQueueReport, CanonicalArguments: canonicalArguments, EvaluatedRevisionID: 1}
	if _, err := NewPostgresReviewFreezer(world.pool).Freeze(ctx, world.run, action, world.scope); err == nil {
		t.Fatal("a review was frozen for a recipient that does not resolve")
	}
	var frozenRows int
	_ = world.pool.QueryRow(ctx, `SELECT count(*) FROM runtime.review_payloads WHERE action_id = $1`, actionID).Scan(&frozenRows)
	if frozenRows != 0 {
		t.Fatalf("%d payload rows for a refused freeze", frozenRows)
	}
}

func TestApprovalRequestEventHoldsNoReviewContent(t *testing.T) {
	world := openReviewWorld(t)
	ctx := context.Background()
	gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool),
		NewPostgresRelationships(world.pool), nil).WithReviewFreezer(NewPostgresReviewFreezer(world.pool))
	world.nextStep++
	decision := gate.Evaluate(ctx, world.run, Proposal{ActionID: testdb.ID(t), StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t),
		Tool: "queue_report", RawArguments: json.RawMessage(`{"report_id":"` + world.reportID + `","recipient_reference":"` + world.reference + `"}`)})
	if decision.Outcome != OutcomeApprovalRequired || decision.Review == nil {
		t.Fatalf("decision = %s/%s, want approval_required with frozen material", decision.Outcome, decision.ReasonCode)
	}
	rows, err := world.pool.Query(ctx, `SELECT event_type, masked_summary::text FROM runtime.audit_events WHERE run_id = $1`, world.run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	sawRequest := false
	for rows.Next() {
		var eventType, summary string
		if err := rows.Scan(&eventType, &summary); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(summary, world.address) || strings.Contains(summary, world.reportBody) {
			t.Fatalf("event %s carries review content: %s", eventType, summary)
		}
		if eventType == "approval.requested" {
			sawRequest = strings.Contains(summary, world.reportID)
		}
	}
	if !sawRequest {
		t.Fatal("no approval.requested event referencing the report")
	}
}
