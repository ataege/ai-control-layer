package policy

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
)

// maximumExpiriesPerCall bounds one ExpireOverdue call; the next call closes the rest.
const maximumExpiriesPerCall = 100

// ExpireOverdue closes every approval nobody decided before its review expired (GO-40). Each one
// commits on its own: the action moves from awaiting_approval to expired, an approval.decided
// event with reason approval_expired is appended, and a continuation job is enqueued for a run
// that is not finished, so the loop can stop it or feed the expiry back. No runtime.approvals row
// is written: an expiry is not a reviewer's decision. Rows another transaction holds (a reviewer
// deciding right now) are skipped, so a decision and an expiry never both land. Calling it again
// closes nothing twice.
func (approvals *Approvals) ExpireOverdue(ctx context.Context) (closed int, err error) {
	return approvals.expireOverdueIn(ctx, "")
}

// expireOverdueIn is ExpireOverdue limited to one organization when organizationID is set, so
// tests on the shared test database never close another test's approvals.
func (approvals *Approvals) expireOverdueIn(ctx context.Context, organizationID string) (closed int, err error) {
	if approvals.pool == nil {
		return 0, ErrApprovalUnavailable
	}
	for closed < maximumExpiriesPerCall {
		expired, err := approvals.expireOne(ctx, organizationID)
		if err != nil {
			return closed, ErrApprovalUnavailable
		}
		if !expired {
			return closed, nil
		}
		closed++
	}
	return closed, nil
}

// expireOne closes the oldest overdue approval in one transaction; false means none was left.
func (approvals *Approvals) expireOne(ctx context.Context, organizationFilter string) (bool, error) {
	expired := false
	err := approvals.repository.InTransaction(ctx, func(tx repository.Tx) error {
		var actionID, organizationID, runID string
		var revisionID int64
		var replaySource *string
		var runFinished bool
		// The action row is locked first, as Decide and the executor do; AppendEvent then takes
		// the run row.
		err := tx.Raw().QueryRow(ctx,
			`SELECT a.id::text, a.organization_id::text, a.run_id::text, a.evaluated_catalog_revision_id, a.replay_source,
			        r.status IN ('completed', 'failed', 'stopped')
			   FROM runtime.actions AS a
			   JOIN runtime.review_payloads AS p ON p.action_id = a.id AND p.organization_id = a.organization_id
			   JOIN runtime.runs AS r ON r.id = a.run_id AND r.organization_id = a.organization_id
			  WHERE a.status = $1 AND p.expires_at <= $2 AND ($3 = '' OR a.organization_id::text = $3)
			  ORDER BY p.expires_at, a.id
			  LIMIT 1
			  FOR UPDATE OF a SKIP LOCKED`,
			string(contracts.ActionAwaitingApproval), approvals.now(), organizationFilter,
		).Scan(&actionID, &organizationID, &runID, &revisionID, &replaySource, &runFinished)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		tag, err := tx.Raw().Exec(ctx,
			`UPDATE runtime.actions SET status = $1, updated_at = now() WHERE id = $2 AND status = $3`,
			string(contracts.ActionExpired), actionID, string(contracts.ActionAwaitingApproval))
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrApprovalUnavailable
		}
		reason := contracts.ReasonApprovalExpired
		if _, err = tx.AppendEvent(ctx, repository.NewEvent{
			OrganizationID: organizationID, RunID: &runID, ActionID: &actionID,
			EventType: contracts.EventApprovalDecided, ReasonCode: &reason, CatalogRevisionID: &revisionID,
			MaskedSummary: contracts.MaskedSummary{ReplaySource: replaySource},
		}); err != nil {
			return err
		}
		if !runFinished {
			if _, err = approvals.enqueue(ctx, tx, organizationID, runID); err != nil {
				return err
			}
		}
		expired = true
		return nil
	})
	return expired, err
}

// DecidedAction is the stored action a continuation resumes: decided by a reviewer or closed by
// expiry, and not executed yet.
type DecidedAction struct {
	ActionID           string
	StepNumber         int
	Tool               ToolName
	CanonicalArguments json.RawMessage
	// Status is approved, rejected or expired.
	Status contracts.ActionStatus
	// GrantOpen is true for an approved action whose grant is unconsumed and unexpired now. The
	// executor rechecks it in its own transaction; this is for the loop's choice only.
	GrantOpen bool
	// GrantExpiresAt is the grant's expiry for an approved action, zero otherwise.
	GrantExpiresAt time.Time
	ReplaySource   *string
}

// DecidedActionFor returns the action the run's continuation resumes: the run's latest action,
// when its status is approved, rejected or expired. Once executed (executing, succeeded, failed,
// unknown) or superseded by a later step, there is nothing to resume and found is false. The run
// is read inside the organization, so another organization's run is not found.
func (approvals *Approvals) DecidedActionFor(ctx context.Context, organizationID, runID string) (DecidedAction, bool, error) {
	if approvals.pool == nil {
		return DecidedAction{}, false, ErrApprovalUnavailable
	}
	if !uuidPattern.MatchString(organizationID) || !uuidPattern.MatchString(runID) {
		return DecidedAction{}, false, ErrApprovalInvalid
	}
	var action DecidedAction
	var tool, status string
	var arguments []byte
	var grantExpiresAt *time.Time
	err := approvals.pool.QueryRow(ctx,
		`SELECT a.id::text, a.step_number, a.tool, a.canonical_arguments::text, a.status, a.replay_source,
		        g.expires_at, (g.id IS NOT NULL AND g.consumed_at IS NULL AND g.expires_at > $3)
		   FROM runtime.actions AS a
		   LEFT JOIN runtime.approvals AS g
		          ON g.action_id = a.id AND g.organization_id = a.organization_id AND g.decision = 'approved'
		  WHERE a.organization_id = $1 AND a.run_id = $2
		  ORDER BY a.step_number DESC
		  LIMIT 1`,
		organizationID, runID, approvals.now(),
	).Scan(&action.ActionID, &action.StepNumber, &tool, &arguments, &status, &action.ReplaySource, &grantExpiresAt, &action.GrantOpen)
	if errors.Is(err, pgx.ErrNoRows) {
		return DecidedAction{}, false, nil
	}
	if err != nil {
		return DecidedAction{}, false, ErrApprovalUnavailable
	}
	action.Status = contracts.ActionStatus(status)
	if action.Status != contracts.ActionApproved && action.Status != contracts.ActionRejected && action.Status != contracts.ActionExpired {
		return DecidedAction{}, false, nil
	}
	action.Tool, action.CanonicalArguments = ToolName(tool), json.RawMessage(arguments)
	if grantExpiresAt != nil {
		action.GrantExpiresAt = *grantExpiresAt
	}
	return action, true, nil
}
