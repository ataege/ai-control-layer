package agent

import (
	"context"
	"testing"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/policy"
)

// TestEveryStopPauseAndFailureIsReadableThroughTheStoredRecords is GO-58's table test: the run's end
// writer records every X-13 reason code with each status that needs a reason, and the stored run
// state and the stored event (what the operator reads) carry the code and its fixed safe message.
func TestEveryStopPauseAndFailureIsReadableThroughTheStoredRecords(t *testing.T) {
	statuses := []contracts.RunStatus{contracts.RunPaused, contracts.RunFailed, contracts.RunStopped}
	checked := 0
	for _, status := range statuses {
		for _, code := range contracts.ReasonCodes {
			t.Run(string(status)+"/"+string(code), func(t *testing.T) {
				world := newLoopWorld(t, passportOptions{})
				stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
				loop := newTestLoop(t, world, stepper)
				run := policy.RunIdentity{OrganizationID: world.organizationID, RunID: world.runID}
				if err := loop.endRun(context.Background(), run, runEnd{status: status, reason: code}); err != nil {
					t.Fatal(err)
				}

				assertRunEnded(t, world, status, code)
				events, err := world.repository.RunEvents(context.Background(), world.organizationID, world.runID, 0, 50)
				if err != nil {
					t.Fatal(err)
				}
				var ended *contracts.SafeEvent
				for index := range events {
					if events[index].EventType == eventFor(status) {
						ended = &events[index]
					}
				}
				if ended == nil || ended.ReasonCode == nil || *ended.ReasonCode != code ||
					ended.MaskedSummary.SafeMessage == nil || *ended.MaskedSummary.SafeMessage != code.SafeMessage() ||
					*ended.MaskedSummary.SafeMessage == "" {
					t.Fatalf("event %+v for %s/%s", ended, status, code)
				}
			})
			checked++
		}
	}
	if checked != len(statuses)*len(contracts.ReasonCodes) {
		t.Fatalf("checked %d pairs", checked)
	}
	t.Logf("evidence GO-58: %d statuses x %d X-13 reason codes = %d run ends: each stored run state carries the code and each stored event "+
		"the code with its fixed safe message", len(statuses), len(contracts.ReasonCodes), checked)
}
