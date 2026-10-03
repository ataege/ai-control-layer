package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
)

// GO-26 in the loop, with the real runresult validator: only a final answer naming reports this
// run created completes the run; anything else is a counted denial.

func answer(text string) func([]model.Message) (StepResult, error) {
	return func([]model.Message) (StepResult, error) { return StepResult{Kind: StepFinal, FinalAnswer: text}, nil }
}

func TestFinalAnswerWithoutACreatedReportIsDeniedAndCounted(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	world.results = PoolFinalResults{Pool: world.pool}
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){
			answer("All done, duplicate reference INV104."),
			answer(`{"status":"completed","report_ids":["5b0f2c1e-8d3a-4c6b-9e7f-1a2b3c4d5e6f"]}`),
			answer("done"),
		}}
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), world.job()); err != nil {
		t.Fatal(err)
	}
	// Two corrections are allowed; the third rejected answer exceeds the limit.
	assertRunEnded(t, world, contracts.RunStopped, contracts.ReasonAllowanceExhausted)
	if denials := world.count(t, "SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1 AND event_type = 'action.denied'"); denials != 3 {
		t.Fatalf("%d denial events, want 3", denials)
	}
	second := stepper.contexts[1]
	if feedback := second[len(second)-1]; feedback.Role != "user" || !strings.Contains(feedback.Content, `"reason_code":"invalid_arguments"`) ||
		!strings.Contains(feedback.Content, "final answer was not accepted") || !strings.Contains(feedback.Content, `\"report_ids\"`) ||
		!strings.Contains(feedback.Content, "no code fence") {
		t.Fatalf("first rejection feedback: %+v", feedback)
	}
	third := stepper.contexts[2]
	if feedback := third[len(third)-1]; !strings.Contains(feedback.Content, `"reason_code":"resource_out_of_scope"`) {
		t.Fatalf("an unknown report id was not rejected as out of scope: %+v", feedback)
	}
	// Each denial stores the fixed cause kind of the rejected answer, never its text.
	rows, err := world.pool.Query(context.Background(), `SELECT coalesce(masked_summary->>'rejectionCause', ''), masked_summary::text
		FROM runtime.audit_events WHERE organization_id = $1 AND event_type = 'action.denied' ORDER BY id`, world.organizationID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var causes []string
	for rows.Next() {
		var cause, summary string
		if err := rows.Scan(&cause, &summary); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(summary, "INV104") || strings.Contains(summary, "done") {
			t.Fatalf("a denial event holds the answer text: %s", summary)
		}
		causes = append(causes, cause)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read rows: %v", err)
	}
	if strings.Join(causes, ",") != "not_json,unknown_report,not_json" {
		t.Fatalf("rejection causes %v, want not_json, unknown_report, not_json", causes)
	}
}

func TestFinalAnswerNamingACreatedReportCompletesWithItsReference(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	world.results = PoolFinalResults{Pool: world.pool}
	createInternalReport := func([]model.Message) (StepResult, error) {
		arguments, _ := json.Marshal(map[string]any{"template": "internal_investigation_v1",
			"source_invoice_ids": []string{world.invoiceWithNote, world.invoiceClean}})
		return StepResult{Kind: StepAction, Proposal: contracts.ActionProposal{Tool: contracts.ToolCreateReport, Arguments: arguments}}, nil
	}
	var reportID string
	nameTheReport := func(messages []model.Message) (StepResult, error) {
		var created struct {
			ReportID string `json:"report_id"`
		}
		_ = json.Unmarshal([]byte(messages[len(messages)-1].Content), &created)
		reportID = created.ReportID
		return StepResult{Kind: StepFinal, FinalAnswer: `{"status":"completed","report_ids":["` + reportID + `"]}`}, nil
	}
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){createInternalReport, nameTheReport}}
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), world.job()); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunCompleted, "")
	state := world.runState(t)
	if reportID == "" || state.ResultReference == nil || len(state.ResultReference.ReportIDs) != 1 || state.ResultReference.ReportIDs[0] != reportID {
		t.Fatalf("result reference %+v for report %q", state.ResultReference, reportID)
	}
}
