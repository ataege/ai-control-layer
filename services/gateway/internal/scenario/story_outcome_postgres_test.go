package scenario

import (
	"strconv"
	"strings"
	"testing"

	"starter/services/gateway/internal/contracts"
)

// liveStoryProblems lists why a finished story run does not count as the legitimate task. The live
// model chooses its own steps, but an infrastructure fault is never a model choice, so a failed run,
// a run paused with an unknown outcome or a call whose usage could not be recorded can never pass.
// A completed run must show its terminal event, an approval, both reports and the one queued message;
// a run stopped at an allowance is a documented model-dependent end and must show its stop event.
func liveStoryProblems(t *testing.T, world *storyWorld, status contracts.RunStatus, reason string, requireLiveVerdicts bool) []string {
	t.Helper()
	runEvents := func(eventType contracts.EventType) int {
		return world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1 AND run_id = $2 AND event_type = $3`,
			world.passport.RunID, string(eventType))
	}
	var problems []string
	switch {
	case status == contracts.RunCompleted:
		if completed := runEvents(contracts.EventRunCompleted); completed != 1 {
			problems = append(problems, "run.completed events "+strconv.Itoa(completed)+", want 1")
		}
		if approvals := runEvents(contracts.EventApprovalRequested); approvals < 1 {
			problems = append(problems, "no approval was requested for the queued report")
		}
		if reports := runEvents(contracts.EventReportCreated); reports < 2 {
			problems = append(problems, "report.created events "+strconv.Itoa(reports)+", want the internal and the vendor report")
		}
		if outbox := world.count(t, `SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1`); outbox != 1 {
			problems = append(problems, "outbox rows "+strconv.Itoa(outbox)+", want exactly 1")
		}
		if requireLiveVerdicts {
			if live := world.count(t, `SELECT count(*) FROM runtime.control_assessments WHERE organization_id = $1 AND verdict_source = 'live'`); live < 1 {
				problems = append(problems, "no live verdict was recorded")
			}
		}
	case status == contracts.RunStopped && (reason == string(contracts.ReasonAllowanceExhausted) || reason == string(contracts.ReasonSecurityAllowanceExhausted)):
		if stopped := runEvents(contracts.EventRunStopped); stopped != 1 {
			problems = append(problems, "run.stopped events "+strconv.Itoa(stopped)+", want 1")
		}
	default:
		problems = append(problems, "run ended "+string(status)+"/"+reason+", want completed or stopped at an allowance")
	}
	if agentCalls := world.count(t, `SELECT count(*) FROM runtime.model_calls WHERE organization_id = $1 AND run_id = $2 AND purpose = 'agent'`,
		world.passport.RunID); agentCalls < 1 {
		problems = append(problems, "no agent model call was recorded")
	}
	if unknown := world.count(t, `SELECT count(*) FROM runtime.model_calls WHERE organization_id = $1 AND run_id = $2 AND outcome = 'usage_unknown'`,
		world.passport.RunID); unknown != 0 {
		problems = append(problems, strconv.Itoa(unknown)+" model calls ended with unknown usage")
	}
	return problems
}

// The outcome check rejects what the live test must never accept: a run that failed, one paused with
// an unknown outcome (the `decision_unavailable` run of the demo database, which no model choice
// causes), and a "completed" run that recorded nothing. It uses the fixture provider's world and
// calls no model.
func TestLiveStoryOutcomeCheckRejectsAnInfrastructureFault(t *testing.T) {
	world := openStory(t, nil)
	for name, outcome := range map[string]struct {
		status contracts.RunStatus
		reason string
	}{
		"failed with decision_unavailable": {contracts.RunFailed, string(contracts.ReasonDecisionUnavailable)},
		"paused with an unknown outcome":   {contracts.RunPaused, string(contracts.ReasonOutcomeUnknown)},
		"completed with nothing recorded":  {contracts.RunCompleted, ""},
		"stopped without its stop event":   {contracts.RunStopped, string(contracts.ReasonAllowanceExhausted)},
		"cancelled":                        {contracts.RunStopped, string(contracts.ReasonRunCancelled)},
	} {
		t.Run(name, func(t *testing.T) {
			problems := liveStoryProblems(t, world, outcome.status, outcome.reason, true)
			if len(problems) == 0 {
				t.Fatalf("the outcome check accepted a run that ended %s/%s with nothing recorded", outcome.status, outcome.reason)
			}
			t.Logf("rejected: %s", strings.Join(problems, "; "))
		})
	}
}
