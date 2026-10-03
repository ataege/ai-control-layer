package policy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/provenance"
	"starter/services/gateway/internal/testdb"
)

// TestApprovalCannotOverrideTheExportRestriction is the X-77 evidence: an Internal only report
// proposed to the correct Atlas recipient is denied at the gate, a submitted approval stores no
// grant, and a replayed grant (inserted directly) still executes nothing.
func TestApprovalCannotOverrideTheExportRestriction(t *testing.T) {
	world := openApprovalWorld(t)
	ctx := context.Background()

	// A genuinely created internal report of this run.
	creatingAction := world.storeAction(t, ToolCreateReport, `{"template":"internal_investigation_v1","source_invoice_ids":["`+world.invoiceID+`"]}`)
	tx, err := world.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	internal, err := provenance.StoreReport(ctx, tx, provenance.NewReport{
		OrganizationID: world.run.OrganizationID, RunID: world.run.RunID, CreatedByActionID: creatingAction,
		Template: provenance.InternalInvestigationV1,
		Sources: []provenance.Source{{Kind: provenance.SourceInvoice, ID: world.invoiceID, Version: 1,
			Classification: provenance.InternalOnly, ConsumedFields: []string{provenance.FieldInvoiceID, provenance.FieldInternalNote}}},
		Title: "Public summary", Content: "Investigation note: INV104 appears twice.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// The proposal to the correct, permitted Atlas recipient is denied before review.
	gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool),
		NewPostgresRelationships(world.pool), nil).WithReviewFreezer(NewPostgresReviewFreezer(world.pool))
	world.nextStep++
	exportAction := testdb.ID(t)
	decision := gate.Evaluate(ctx, world.run, Proposal{ActionID: exportAction, StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t),
		Tool: "queue_report", RawArguments: json.RawMessage(`{"report_id":"` + internal.ID + `","recipient_reference":"` + world.reference + `"}`)})
	if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonReportExportRestricted {
		t.Fatalf("gate decision = %s/%s, want deny/report_export_restricted", decision.Outcome, decision.ReasonCode)
	}

	// A submitted approval for the denied export stores no grant.
	if _, err := NewApprovals(world.pool).Decide(ctx, world.reviewer, exportAction, ApprovalApprove); err == nil {
		t.Fatal("an approval for the denied export was accepted")
	} else if !errors.Is(err, ErrApprovalNotFound) && !errors.Is(err, ErrApprovalClosed) {
		t.Fatalf("approval err = %v", err)
	}

	// A replayed grant, inserted as the owner, with the action set to approved, executes nothing.
	var digest []byte
	if err := world.pool.QueryRow(ctx, `SELECT action_digest FROM runtime.actions WHERE id = $1`, exportAction).Scan(&digest); err != nil {
		t.Fatal(err)
	}
	mustExec(t, world.pool, `INSERT INTO runtime.approvals (organization_id, action_id, action_digest, review_payload_reference,
	                           reviewer_id, decision, expires_at) VALUES ($1, $2, $3, 'replayed', $4, 'approved', now() + interval '10 minutes')`,
		world.run.OrganizationID, exportAction, digest, world.reviewer.UserID)
	mustExec(t, world.pool, `UPDATE runtime.actions SET status = 'approved' WHERE id = $1`, exportAction)
	result := world.executor(1).Execute(ctx, world.run, exportAction)
	var outboxRows int
	mustScan(t, world.pool.QueryRow(ctx, `SELECT count(*) FROM demo.outbox_messages WHERE report_id = $1`, internal.ID), &outboxRows)
	if result.Status == ExecutionSucceeded || outboxRows != 0 {
		t.Fatalf("replayed grant: result %s/%s, outbox rows %d", result.Status, result.ReasonCode, outboxRows)
	}
	t.Logf("evidence X-77: Internal only report to the correct Atlas recipient -> %s before review; submitted approval refused; replayed grant -> %s/%s; outbox rows for the report: %d",
		decision.ReasonCode, result.Status, result.ReasonCode, outboxRows)
	_ = contracts.ReasonReportExportRestricted
}
