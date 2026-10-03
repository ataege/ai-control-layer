package policy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/testdb"
)

// approvalWorld is a review world with an action awaiting approval and an organization whose
// reviewer and plain operator are real memberships.
type approvalWorld struct {
	*reviewWorld
	actionID string
	reviewer contracts.OperatorContext
	operator contracts.OperatorContext
}

func openApprovalWorld(t *testing.T) *approvalWorld {
	t.Helper()
	world := &approvalWorld{reviewWorld: openReviewWorld(t)}
	ctx := context.Background()
	suffix := testdb.ID(t)[:8]
	mustExec(t, world.pool, `INSERT INTO app.organizations (id, name) VALUES ($1, $2)`, world.run.OrganizationID, "Org "+suffix)
	world.reviewer = world.addMember(t, []string{"operator", "reviewer"})
	world.operator = world.addMember(t, []string{"operator"})

	gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool),
		NewPostgresRelationships(world.pool), nil).WithReviewFreezer(NewPostgresReviewFreezer(world.pool))
	world.nextStep++
	world.actionID = testdb.ID(t)
	decision := gate.Evaluate(ctx, world.run, Proposal{ActionID: world.actionID, StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t),
		Tool: "queue_report", RawArguments: json.RawMessage(`{"report_id":"` + world.reportID + `","recipient_reference":"` + world.reference + `"}`)})
	if decision.Outcome != OutcomeApprovalRequired {
		t.Fatalf("setup decision = %s/%s, want approval_required", decision.Outcome, decision.ReasonCode)
	}
	return world
}

// addMember creates a user with a membership in the world's organization and returns its context.
func (world *approvalWorld) addMember(t *testing.T, roles []string) contracts.OperatorContext {
	t.Helper()
	userID := testdb.ID(t)
	mustExec(t, world.pool, `INSERT INTO app.users (id, email, name) VALUES ($1, $2, 'Member')`, userID, userID+"@example.test")
	mustExec(t, world.pool, `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, $3)`,
		userID, world.run.OrganizationID, roles)
	return contracts.OperatorContext{UserID: userID, OrganizationID: world.run.OrganizationID, Roles: roles}
}

func (world *approvalWorld) counts(t *testing.T) (approvals, jobs int, status string) {
	t.Helper()
	ctx := context.Background()
	_ = world.pool.QueryRow(ctx, `SELECT count(*) FROM runtime.approvals WHERE action_id = $1`, world.actionID).Scan(&approvals)
	_ = world.pool.QueryRow(ctx, `SELECT count(*) FROM runtime.jobs WHERE run_id = $1`, world.run.RunID).Scan(&jobs)
	_ = world.pool.QueryRow(ctx, `SELECT status FROM runtime.actions WHERE id = $1`, world.actionID).Scan(&status)
	return approvals, jobs, status
}

func TestReviewerApprovalStoresTheGrantAndTheContinuationTogether(t *testing.T) {
	world := openApprovalWorld(t)
	ctx := context.Background()
	result, err := NewApprovals(world.pool).Decide(ctx, world.reviewer, world.actionID, ApprovalApprove)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	approvals, jobs, status := world.counts(t)
	if approvals != 1 || jobs != 1 || status != string(contracts.ActionApproved) || result.JobID == "" || result.RunID != world.run.RunID {
		t.Fatalf("approvals %d, jobs %d, status %s, result %+v", approvals, jobs, status, result)
	}
	var decision, reviewer, payloadReference string
	var expiresAt time.Time
	if err := world.pool.QueryRow(ctx, `SELECT decision, reviewer_id::text, review_payload_reference, expires_at FROM runtime.approvals WHERE action_id = $1`,
		world.actionID).Scan(&decision, &reviewer, &payloadReference, &expiresAt); err != nil {
		t.Fatal(err)
	}
	if decision != "approved" || reviewer != world.reviewer.UserID || payloadReference == "" || !expiresAt.Equal(world.scope.ExpiresAt) {
		t.Fatalf("approval row %s %s %q %s", decision, reviewer, payloadReference, expiresAt)
	}
	var eventType string
	if err := world.pool.QueryRow(ctx, `SELECT event_type FROM runtime.audit_events WHERE action_id = $1 ORDER BY id DESC LIMIT 1`, world.actionID).
		Scan(&eventType); err != nil || eventType != "approval.decided" {
		t.Fatalf("last event = %q, err %v", eventType, err)
	}
	// A decision on a closed approval fails and stores nothing more.
	if _, err := NewApprovals(world.pool).Decide(ctx, world.reviewer, world.actionID, ApprovalReject); !errors.Is(err, ErrApprovalClosed) {
		t.Fatalf("second decision err = %v, want ErrApprovalClosed", err)
	}
	if approvals, jobs, _ := world.counts(t); approvals != 1 || jobs != 1 {
		t.Fatalf("after a second decision: approvals %d, jobs %d", approvals, jobs)
	}
}

func TestRejectionClosesTheApprovalAndContinues(t *testing.T) {
	world := openApprovalWorld(t)
	if _, err := NewApprovals(world.pool).Decide(context.Background(), world.reviewer, world.actionID, ApprovalReject); err != nil {
		t.Fatal(err)
	}
	if approvals, jobs, status := world.counts(t); approvals != 1 || jobs != 1 || status != string(contracts.ActionRejected) {
		t.Fatalf("approvals %d, jobs %d, status %s", approvals, jobs, status)
	}
}

func TestApprovalRefusalsStoreNoGrant(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*testing.T, *approvalWorld) (*Approvals, contracts.OperatorContext)
		wantErr error
	}{
		{"not a reviewer", func(t *testing.T, world *approvalWorld) (*Approvals, contracts.OperatorContext) {
			return NewApprovals(world.pool), world.operator
		}, ErrNotReviewer},
		{"signed reviewer role without a membership role", func(t *testing.T, world *approvalWorld) (*Approvals, contracts.OperatorContext) {
			claimed := world.operator
			claimed.Roles = []string{"reviewer"} // the claim alone never suffices
			return NewApprovals(world.pool), claimed
		}, ErrNotReviewer},
		{"reviewer of another organization", func(t *testing.T, world *approvalWorld) (*Approvals, contracts.OperatorContext) {
			otherOrganization := testdb.ID(t)
			mustExec(t, world.pool, `INSERT INTO app.organizations (id, name) VALUES ($1, $2)`, otherOrganization, "Other "+otherOrganization[:8])
			userID := testdb.ID(t)
			mustExec(t, world.pool, `INSERT INTO app.users (id, email, name) VALUES ($1, $2, 'Other')`, userID, userID+"@example.test")
			mustExec(t, world.pool, `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, '{reviewer}')`, userID, otherOrganization)
			return NewApprovals(world.pool), contracts.OperatorContext{UserID: userID, OrganizationID: otherOrganization, Roles: []string{"reviewer"}}
		}, ErrApprovalNotFound},
		{"expired approval", func(t *testing.T, world *approvalWorld) (*Approvals, contracts.OperatorContext) {
			approvals := NewApprovals(world.pool)
			approvals.now = func() time.Time { return world.scope.ExpiresAt.Add(time.Second) }
			return approvals, world.reviewer
		}, ErrApprovalExpired},
		{"altered action", func(t *testing.T, world *approvalWorld) (*Approvals, contracts.OperatorContext) {
			mustExec(t, world.pool, `UPDATE runtime.actions SET canonical_arguments = $1 WHERE id = $2`,
				`{"report_id":"`+world.reportID+`","recipient_reference":"recipient:`+world.run.RunID+`:vendor_other"}`, world.actionID)
			return NewApprovals(world.pool), world.reviewer
		}, ErrApprovalChanged},
		{"cancel-stamped run", func(t *testing.T, world *approvalWorld) (*Approvals, contracts.OperatorContext) {
			mustExec(t, world.pool, `UPDATE runtime.runs SET cancel_requested_at = now() WHERE id = $1`, world.run.RunID)
			return NewApprovals(world.pool), world.reviewer
		}, ErrApprovalRunStopped},
		{"stopped run", func(t *testing.T, world *approvalWorld) (*Approvals, contracts.OperatorContext) {
			mustExec(t, world.pool, `UPDATE runtime.runs SET status = 'stopped', terminal_reason = 'run_cancelled' WHERE id = $1`, world.run.RunID)
			return NewApprovals(world.pool), world.reviewer
		}, ErrApprovalRunStopped},
		{"continuation cannot be stored", func(t *testing.T, world *approvalWorld) (*Approvals, contracts.OperatorContext) {
			approvals := NewApprovals(world.pool)
			approvals.enqueue = func(context.Context, repository.Tx, string, string) (string, error) {
				return "", errors.New("injected continuation failure")
			}
			return approvals, world.reviewer
		}, ErrApprovalUnavailable},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			world := openApprovalWorld(t)
			approvals, operator := testCase.prepare(t, world)
			if _, err := approvals.Decide(context.Background(), operator, world.actionID, ApprovalApprove); !errors.Is(err, testCase.wantErr) {
				t.Fatalf("err = %v, want %v", err, testCase.wantErr)
			}
			if approvals, jobs, status := world.counts(t); approvals != 0 || jobs != 0 || status != string(contracts.ActionAwaitingApproval) {
				t.Fatalf("after a refusal: approvals %d, jobs %d, status %s", approvals, jobs, status)
			}
		})
	}
}

func TestFrozenReviewIsReadOnlyByReviewers(t *testing.T) {
	world := openApprovalWorld(t)
	ctx := context.Background()
	payload, err := NewApprovals(world.pool).FrozenReviewFor(ctx, world.reviewer, world.actionID)
	if err != nil || payload.Recipient == nil || payload.Recipient.Address != world.address || payload.Report == nil || payload.Report.Content != world.reportBody {
		t.Fatalf("review payload = %+v, err %v", payload, err)
	}
	if _, err := NewApprovals(world.pool).FrozenReviewFor(ctx, world.operator, world.actionID); !errors.Is(err, ErrNotReviewer) {
		t.Fatalf("operator read err = %v, want ErrNotReviewer", err)
	}
}

func TestGatewayRoleReadsMembershipsButCannotWriteThem(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE task_passport_gateway`); err != nil {
		t.Fatalf("set role: %v (run the migrations)", err)
	}
	if _, err := tx.Exec(ctx, `SELECT count(*) FROM app.memberships`); err != nil {
		t.Fatalf("the gateway role cannot read memberships: %v", err)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT write_attempt`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.memberships SET roles = '{reviewer}'`); err == nil {
		t.Fatal("the gateway role could write memberships")
	}
}
