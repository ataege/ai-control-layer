package policy

import (
	"context"
	"encoding/json"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

// config/README.md, "Reports and audit": removing a template from reports.enabled_templates forbids
// new reports from it, and a queue attempt for an existing report from that template is denied
// (template_not_allowed) instead of being sent to review.
func TestQueueOfAReportFromADisabledTemplateIsDeniedBeforeReview(t *testing.T) {
	world := openReviewWorld(t)
	ctx := context.Background()
	mustExec(t, world.pool, `INSERT INTO app.organizations (id, name) VALUES ($1, $2)`, world.run.OrganizationID, "Org "+testdb.ID(t)[:8])
	queue := func(snapshots fakeSnapshots) (Decision, string) {
		gate := NewGate(scopeReaderOn(world, snapshots), NewPostgresRecorder(world.pool), NewPostgresRelationships(world.pool), nil).
			WithReviewFreezer(NewPostgresReviewFreezer(world.pool))
		world.nextStep++
		actionID := testdb.ID(t)
		decision := gate.Evaluate(ctx, world.run, Proposal{ActionID: actionID, StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t),
			Tool: "queue_report", RawArguments: json.RawMessage(`{"report_id":"` + world.reportID + `","recipient_reference":"` + world.reference + `"}`)})
		var status string
		mustScan(t, world.pool.QueryRow(ctx, `SELECT status FROM runtime.actions WHERE id = $1`, actionID), &status)
		return decision, status
	}

	// The report's template is still enabled: the queue proposal goes to review, as before.
	decision, status := queue(enabling(2, contracts.TemplateVendorReconciliation))
	if decision.Outcome != OutcomeApprovalRequired || status != actionStatusAwaitingApproval {
		t.Fatalf("template enabled: %s/%s, stored status %s, want approval_required awaiting review", decision.Outcome, decision.ReasonCode, status)
	}

	// The judge disables the report's template; the same proposal is denied and it is not left awaiting review.
	decision, status = queue(enabling(3, contracts.TemplateInternalInvestigation))
	if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonTemplateNotAllowed || status == actionStatusAwaitingApproval {
		t.Fatalf("template disabled: %s/%s, stored status %s, want deny/%s and not awaiting review", decision.Outcome, decision.ReasonCode, status, ReasonTemplateNotAllowed)
	}
}
