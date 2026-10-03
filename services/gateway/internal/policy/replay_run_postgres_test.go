package policy

import (
	"context"
	"errors"
	"testing"

	"starter/services/gateway/internal/testdb"
)

// finishRun marks the world's run completed, as the agent loop does when it ends.
func finishRun(t *testing.T, world *approvalWorld) {
	t.Helper()
	mustExec(t, world.pool, `UPDATE runtime.runs SET status = 'completed', updated_at = now() WHERE id = $1`, world.run.RunID)
}

func countRunActions(t *testing.T, world *approvalWorld) (actions, lastStep int) {
	t.Helper()
	if err := world.pool.QueryRow(context.Background(),
		`SELECT count(*), coalesce(max(step_number), 0) FROM runtime.actions WHERE run_id = $1`, world.run.RunID).Scan(&actions, &lastStep); err != nil {
		t.Fatal(err)
	}
	return actions, lastStep
}

// TestDemoReplayOfAFinishedRunIsDeniedAndLabelled drives each fixture the way cmd/replay does:
// the runner reads the run's own records, takes the next step and submits the labelled proposal
// to the gate, which stores the denial with the replay label.
func TestDemoReplayOfAFinishedRunIsDeniedAndLabelled(t *testing.T) {
	for _, fixtureID := range ReplayFixtureIDs() {
		t.Run(fixtureID, func(t *testing.T) {
			ctx := context.Background()
			world := openApprovalWorld(t)
			replayTargets(t, world) // adds the run's Internal only report
			finishRun(t, world)
			_, lastStep := countRunActions(t, world)
			gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool),
				NewPostgresRelationships(world.pool), nil).WithReviewFreezer(NewPostgresReviewFreezer(world.pool))

			outcome, err := NewReplayRunner(world.pool, gate).Replay(ctx, world.run.RunID, fixtureID)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Decision.Outcome != OutcomeDeny || outcome.Decision.ReasonCode != outcome.ExpectedReason ||
				outcome.StepNumber != lastStep+1 || outcome.Run != world.run {
				t.Fatalf("outcome %+v (decision %s/%s), want deny/%s at step %d", outcome, outcome.Decision.Outcome,
					outcome.Decision.ReasonCode, outcome.ExpectedReason, lastStep+1)
			}
			var storedLabel, eventLabel string
			if err := world.pool.QueryRow(ctx, `SELECT replay_source FROM runtime.actions WHERE id = $1`, outcome.ActionID).Scan(&storedLabel); err != nil {
				t.Fatal(err)
			}
			if err := world.pool.QueryRow(ctx, `SELECT masked_summary->>'replaySource' FROM runtime.audit_events WHERE action_id = $1`,
				outcome.ActionID).Scan(&eventLabel); err != nil {
				t.Fatal(err)
			}
			if storedLabel != ReplaySourcePrefix+fixtureID || eventLabel != storedLabel {
				t.Fatalf("labels: action %q, event %q", storedLabel, eventLabel)
			}
		})
	}
}

func TestDemoReplayRefusesWithoutWriting(t *testing.T) {
	ctx := context.Background()
	world := openApprovalWorld(t)
	gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool), NewPostgresRelationships(world.pool), nil)
	runner := NewReplayRunner(world.pool, gate)
	before, _ := countRunActions(t, world)

	// A live run: the replay could take the step the loop uses next.
	if _, err := runner.Replay(ctx, world.run.RunID, "hostile_note_redirect_record_v1"); !errors.Is(err, ErrReplayRunNotFinished) {
		t.Fatalf("live run: err = %v", err)
	}
	finishRun(t, world)
	cases := []struct {
		name, runID, fixtureID string
		want                   error
	}{
		{"unknown fixture", world.run.RunID, "hostile_note_unknown_v1", ErrUnknownReplayFixture},
		{"unknown run", testdb.ID(t), "hostile_note_redirect_record_v1", ErrReplayRunNotFound},
		{"malformed run id", "not-a-run", "hostile_note_redirect_record_v1", ErrReplayRunNotFound},
		{"no Internal only report in the run", world.run.RunID, "hostile_note_internal_disclosure_v1", ErrReplayTargetMissing},
	}
	for _, testCase := range cases {
		if _, err := runner.Replay(ctx, testCase.runID, testCase.fixtureID); !errors.Is(err, testCase.want) {
			t.Fatalf("%s: err = %v, want %v", testCase.name, err, testCase.want)
		}
	}
	if after, _ := countRunActions(t, world); after != before {
		t.Fatalf("refused replays stored actions: %d -> %d", before, after)
	}
}
