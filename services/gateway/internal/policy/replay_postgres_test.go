package policy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"starter/services/gateway/internal/provenance"
	"starter/services/gateway/internal/testdb"
	"starter/services/gateway/internal/tools"
)

// TestReplayFixturesExistInTheHostileNotes keeps the replay table in step with X-34.
func TestReplayFixturesExistInTheHostileNotes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "fixtures", "hostile-notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Notes []struct {
			ID     string `json:"id"`
			Reason string `json:"deterministic_reason_if_obeyed"`
		} `json:"notes"`
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	reasons := map[string]string{}
	for _, note := range fixtures.Notes {
		reasons[note.ID] = note.Reason
	}
	for _, fixtureID := range ReplayFixtureIDs() {
		if reasons[fixtureID] != string(replayFixtures[fixtureID].expectedReason) {
			t.Fatalf("fixture %s: hostile-notes.json reason %q, replay expects %q", fixtureID, reasons[fixtureID], replayFixtures[fixtureID].expectedReason)
		}
	}
}

// replayTargets adds the records the hostile notes point at to an approval world: an invoice of the
// organization outside the passport and an Internal only report of the run.
func replayTargets(t *testing.T, world *approvalWorld) ReplayTargets {
	t.Helper()
	ctx := context.Background()
	outOfScope := "invoice_B01_" + testdb.ID(t)[:8]
	var vendorID string
	if err := world.pool.QueryRow(ctx, `SELECT vendor_id FROM demo.invoices WHERE id = $1`, world.invoiceID).Scan(&vendorID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, world.pool, `INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency, total_minor_units, issued_on, due_on)
	                         VALUES ($1, $2, $3, 'INV211', 'EUR', 48000, '2026-09-15', '2026-11-15')`, outOfScope, world.run.OrganizationID, vendorID)
	creating := world.storeAction(t, ToolCreateReport, `{"template":"internal_investigation_v1","source_invoice_ids":["`+world.invoiceID+`"]}`)
	tx, err := world.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	internal, err := provenance.StoreReport(ctx, tx, provenance.NewReport{
		OrganizationID: world.run.OrganizationID, RunID: world.run.RunID, CreatedByActionID: creating,
		Template: provenance.InternalInvestigationV1,
		Sources: []provenance.Source{{Kind: provenance.SourceInvoice, ID: world.invoiceID, Version: 1,
			Classification: provenance.InternalOnly, ConsumedFields: []string{provenance.FieldInvoiceID, provenance.FieldInternalNote}}},
		Title: "Investigation", Content: "Investigation note: INV104 appears twice.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return ReplayTargets{OutOfScopeInvoiceID: outOfScope, VendorReportID: world.reportID, InternalReportID: internal.ID, RecipientReference: world.reference}
}

// TestLabelledReplayIsDeniedByTheRealGate is the X-36 evidence: each replayed prohibited proposal
// runs through the production gate and executor, is denied with the same reason as its live
// equivalent, leaves no business effect, and every record of it carries the replay label.
func TestLabelledReplayIsDeniedByTheRealGate(t *testing.T) {
	for _, fixtureID := range ReplayFixtureIDs() {
		t.Run(fixtureID, func(t *testing.T) {
			ctx := context.Background()
			world := openApprovalWorld(t)
			targets := replayTargets(t, world)
			gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool),
				NewPostgresRelationships(world.pool), nil).WithReviewFreezer(NewPostgresReviewFreezer(world.pool))
			executor := NewExecutor(world.pool, &fakeScopes{scope: world.scope, revision: 1}, tools.Runner{})

			// The replay: the next step of the run.
			world.nextStep++
			replayAction := testdb.ID(t)
			replayed, expectedReason, err := ReplayProposal(fixtureID, targets, replayAction, world.nextStep, testdb.ID(t))
			if err != nil {
				t.Fatal(err)
			}
			replayDecision := gate.Evaluate(ctx, world.run, replayed)
			replayExecution := executor.Execute(ctx, world.run, replayAction)

			// Its live equivalent: the same proposal without the label, as a model would send it.
			world.nextStep++
			liveProposal := replayed
			liveProposal.ActionID, liveProposal.StepNumber, liveProposal.IdempotencyKey, liveProposal.ReplaySource = testdb.ID(t), world.nextStep, testdb.ID(t), ""
			liveDecision := gate.Evaluate(ctx, world.run, liveProposal)

			if replayDecision.Outcome != OutcomeDeny || replayDecision.ReasonCode != expectedReason || liveDecision.ReasonCode != replayDecision.ReasonCode {
				t.Fatalf("replay %s/%s, live %s/%s, want deny/%s for both", replayDecision.Outcome, replayDecision.ReasonCode, liveDecision.Outcome, liveDecision.ReasonCode, expectedReason)
			}
			if replayExecution.Status != ExecutionRefused {
				t.Fatalf("executing the denied replay = %s, want refused", replayExecution.Status)
			}

			var storedLabel *string
			mustScan(t, world.pool.QueryRow(ctx, `SELECT replay_source FROM runtime.actions WHERE id = $1`, replayAction), &storedLabel)
			label := ReplaySourcePrefix + fixtureID
			if storedLabel == nil || *storedLabel != label {
				t.Fatalf("stored replay label = %v, want %s", storedLabel, label)
			}
			rows, err := world.pool.Query(ctx, `SELECT event_type, masked_summary::text FROM runtime.audit_events WHERE action_id = $1`, replayAction)
			if err != nil {
				t.Fatal(err)
			}
			events := 0
			for rows.Next() {
				var eventType, summary string
				if err := rows.Scan(&eventType, &summary); err != nil {
					t.Fatal(err)
				}
				events++
				if !strings.Contains(summary, `"replaySource": "`+label+`"`) {
					t.Fatalf("event %s of the replay lacks the label: %s", eventType, summary)
				}
			}
			rows.Close()
			var liveLabel *string
			mustScan(t, world.pool.QueryRow(ctx, `SELECT replay_source FROM runtime.actions WHERE id = $1`, liveProposal.ActionID), &liveLabel)

			var outbox, attempts, modelCalls int
			mustScan(t, world.pool.QueryRow(ctx, `SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1`, world.run.OrganizationID), &outbox)
			mustScan(t, world.pool.QueryRow(ctx, `SELECT count(*) FROM runtime.execution_attempts WHERE action_id = $1`, replayAction), &attempts)
			mustScan(t, world.pool.QueryRow(ctx, `SELECT count(*) FROM runtime.model_calls WHERE run_id = $1`, world.run.RunID), &modelCalls)
			if events == 0 || outbox != 0 || attempts != 0 || modelCalls != 0 || liveLabel != nil {
				t.Fatalf("events %d, outbox %d, attempts %d, model calls %d, live label %v", events, outbox, attempts, modelCalls, liveLabel)
			}
			t.Logf("evidence X-36: LABELLED REPLAY %s -> deny %s (live equivalent: %s); executor %s; %d event(s), all labelled; outbox 0, attempts 0, model calls 0",
				label, replayDecision.ReasonCode, liveDecision.ReasonCode, replayExecution.Status, events)
		})
	}
}
