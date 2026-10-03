package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
)

// ReviewerRole is the membership role an operator needs to decide an approval (lead decision).
const ReviewerRole = "reviewer"

// ApprovalChoice is the X-10 decision: approve or reject one stored action, nothing else. It is
// handler-local until 3c adds ApprovalDecision to internal/contracts.
type ApprovalChoice string

const (
	ApprovalApprove ApprovalChoice = "approve"
	ApprovalReject  ApprovalChoice = "reject"
)

// Approval decision failures. Each one stores no grant.
var (
	ErrNotReviewer         = errors.New("operator is not a reviewer of this organization")
	ErrApprovalNotFound    = errors.New("no action awaiting approval")
	ErrApprovalClosed      = errors.New("approval already decided")
	ErrApprovalExpired     = errors.New("approval expired")
	ErrApprovalChanged     = errors.New("action or review material changed")
	ErrApprovalInvalid     = errors.New("invalid approval decision")
	ErrApprovalUnavailable = errors.New("approval could not be decided")
)

// ApprovalResult is what a decided approval stored.
type ApprovalResult struct {
	ApprovalID string
	ActionID   string
	RunID      string
	Decision   ApprovalChoice
	JobID      string
}

// ContinuationEnqueuer enqueues the job that resumes the run after a decision, inside the
// decision's transaction. The default is the runtime repository's EnqueueJob.
type ContinuationEnqueuer func(ctx context.Context, tx repository.Tx, organizationID, runID string) (string, error)

// Approvals decides exact-action approvals (GO-44).
type Approvals struct {
	pool       *pgxpool.Pool
	repository *repository.Repository
	enqueue    ContinuationEnqueuer
	now        func() time.Time
}

// NewApprovals returns the approval manager. It creates nothing at construction.
func NewApprovals(pool *pgxpool.Pool) *Approvals {
	return &Approvals{
		pool: pool, repository: repository.New(pool), now: time.Now,
		enqueue: func(ctx context.Context, tx repository.Tx, organizationID, runID string) (string, error) {
			return tx.EnqueueJob(ctx, organizationID, runID, contracts.JobKindAgentStep)
		},
	}
}

// awaitingAction is the stored action and its frozen review material, read under a lock.
type awaitingAction struct {
	runID, passportID   string
	tool                ToolName
	canonicalArguments  []byte
	actionDigest        []byte
	evaluatedRevisionID int64
	status              string
	payloadID           string
	payload             []byte
	payloadDigest       []byte
	expiresAt           time.Time
}

// Decide checks the reviewer's authority from app.memberships (never from the signed claim
// alone), the action's integrity and the frozen material's digest and expiry, then stores the
// grant and the continuation in one transaction: the approvals row, the action's status, the
// approval.decided event and the continuation job, or none of them. An approval never changes
// what the gate denied: only an action awaiting approval can be decided.
func (approvals *Approvals) Decide(ctx context.Context, operator contracts.OperatorContext, actionID string, choice ApprovalChoice) (ApprovalResult, error) {
	if approvals.pool == nil {
		return ApprovalResult{}, ErrApprovalUnavailable
	}
	if (choice != ApprovalApprove && choice != ApprovalReject) || !uuidPattern.MatchString(actionID) ||
		!uuidPattern.MatchString(operator.UserID) || !uuidPattern.MatchString(operator.OrganizationID) {
		return ApprovalResult{}, ErrApprovalInvalid
	}
	var result ApprovalResult
	err := approvals.repository.InTransaction(ctx, func(tx repository.Tx) error {
		if err := requireReviewer(ctx, tx.Raw(), operator); err != nil {
			return err
		}
		action, err := loadAwaitingAction(ctx, tx.Raw(), operator.OrganizationID, actionID)
		if err != nil {
			return err
		}
		if !action.expiresAt.After(approvals.now()) {
			return ErrApprovalExpired
		}
		if !awaitingActionIntact(actionID, action) {
			return ErrApprovalChanged
		}

		decision, finalStatus, eventDecision := "approved", contracts.ActionApproved, contracts.DecisionApproved
		if choice == ApprovalReject {
			decision, finalStatus, eventDecision = "rejected", contracts.ActionRejected, contracts.DecisionRejected
		}
		if err := tx.Raw().QueryRow(ctx,
			`INSERT INTO runtime.approvals
			   (organization_id, action_id, action_digest, review_payload_reference, reviewer_id, decision, expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id::text`,
			operator.OrganizationID, actionID, action.actionDigest, action.payloadID, operator.UserID, decision, action.expiresAt,
		).Scan(&result.ApprovalID); err != nil {
			return ErrApprovalClosed // the one-per-action constraint: a decision already exists
		}
		tag, err := tx.Raw().Exec(ctx,
			`UPDATE runtime.actions SET status = $1, updated_at = now()
			  WHERE id = $2 AND organization_id = $3 AND status = $4`,
			string(finalStatus), actionID, operator.OrganizationID, string(contracts.ActionAwaitingApproval))
		if err != nil || tag.RowsAffected() != 1 {
			return ErrApprovalClosed
		}
		runID, revisionID := action.runID, action.evaluatedRevisionID
		reference := actionID
		if _, err := tx.AppendEvent(ctx, repository.NewEvent{
			OrganizationID: operator.OrganizationID, RunID: &runID, ActionID: &reference,
			EventType: contracts.EventApprovalDecided, Decision: &eventDecision, CatalogRevisionID: &revisionID,
		}); err != nil {
			return err
		}
		jobID, err := approvals.enqueue(ctx, tx, operator.OrganizationID, action.runID)
		if err != nil {
			return err
		}
		result = ApprovalResult{ApprovalID: result.ApprovalID, ActionID: actionID, RunID: action.runID, Decision: choice, JobID: jobID}
		return nil
	})
	switch {
	case err == nil:
		return result, nil
	case errors.Is(err, ErrNotReviewer), errors.Is(err, ErrApprovalNotFound), errors.Is(err, ErrApprovalClosed),
		errors.Is(err, ErrApprovalExpired), errors.Is(err, ErrApprovalChanged):
		return ApprovalResult{}, err
	default:
		return ApprovalResult{}, ErrApprovalUnavailable
	}
}

// requireReviewer reads the verified operator's membership in the organization; a missing row, a
// failed read or a membership without the reviewer role denies.
func requireReviewer(ctx context.Context, tx pgx.Tx, operator contracts.OperatorContext) error {
	rows, err := tx.Query(ctx,
		`SELECT roles FROM app.memberships WHERE "userId" = $1 AND "organizationId" = $2`,
		operator.UserID, operator.OrganizationID)
	if err != nil {
		return ErrApprovalUnavailable
	}
	defer rows.Close()
	isReviewer := false
	for rows.Next() {
		var roles []string
		if err := rows.Scan(&roles); err != nil {
			return ErrApprovalUnavailable
		}
		isReviewer = isReviewer || slices.Contains(roles, ReviewerRole)
	}
	if rows.Err() != nil {
		return ErrApprovalUnavailable
	}
	if !isReviewer {
		return ErrNotReviewer
	}
	return nil
}

// loadAwaitingAction locks the action of the organization and reads its frozen review material.
// An action of another organization is indistinguishable from a missing one.
func loadAwaitingAction(ctx context.Context, tx pgx.Tx, organizationID, actionID string) (awaitingAction, error) {
	var action awaitingAction
	var tool string
	err := tx.QueryRow(ctx,
		`SELECT a.run_id::text, r.passport_id::text, a.tool, a.canonical_arguments::text, a.action_digest,
		        a.evaluated_catalog_revision_id, a.status, p.id::text, p.payload::text, p.payload_digest, p.expires_at
		   FROM runtime.actions AS a
		   JOIN runtime.runs AS r ON r.id = a.run_id AND r.organization_id = a.organization_id
		   JOIN runtime.review_payloads AS p ON p.action_id = a.id AND p.organization_id = a.organization_id
		  WHERE a.id = $1 AND a.organization_id = $2
		  FOR UPDATE OF a`,
		actionID, organizationID,
	).Scan(&action.runID, &action.passportID, &tool, &action.canonicalArguments, &action.actionDigest,
		&action.evaluatedRevisionID, &action.status, &action.payloadID, &action.payload, &action.payloadDigest, &action.expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return awaitingAction{}, ErrApprovalNotFound
	}
	if err != nil {
		return awaitingAction{}, ErrApprovalUnavailable
	}
	action.tool = ToolName(tool)
	if action.status != string(contracts.ActionAwaitingApproval) {
		return awaitingAction{}, ErrApprovalClosed
	}
	return action, nil
}

// awaitingActionIntact recomputes the action's digest from its stored arguments and the frozen
// payload's digest from its stored content; either mismatch means the reviewed material changed.
func awaitingActionIntact(actionID string, action awaitingAction) bool {
	arguments, err := DecodeArguments(action.tool, action.canonicalArguments)
	if err != nil {
		return false
	}
	actionDigest, err := CanonicalAction{
		ActionID: actionID, RunID: action.runID, Arguments: arguments,
		PassportID: action.passportID, PolicyRevisionID: action.evaluatedRevisionID,
	}.Digest()
	if err != nil || !bytes.Equal(actionDigest[:], action.actionDigest) {
		return false
	}
	var payload ReviewPayload
	if json.Unmarshal(action.payload, &payload) != nil || payload.ActionID != actionID {
		return false
	}
	payloadDigest, _, err := payload.Digest()
	if err != nil || !bytes.Equal(payloadDigest[:], action.payloadDigest) {
		return false
	}
	// The frozen arguments must be the stored action's arguments.
	frozenArguments, err := DecodeArguments(payload.Tool, payload.CanonicalArguments)
	if err != nil || payload.Tool != action.tool {
		return false
	}
	frozenCanonical, frozenErr := CanonicalArguments(frozenArguments)
	storedCanonical, storedErr := CanonicalArguments(arguments)
	return frozenErr == nil && storedErr == nil && bytes.Equal(frozenCanonical, storedCanonical)
}

// FrozenReviewFor returns the frozen review payload of an action of the operator's organization,
// for an authorized reviewer only (the review screen's read, GO-44).
func (approvals *Approvals) FrozenReviewFor(ctx context.Context, operator contracts.OperatorContext, actionID string) (ReviewPayload, error) {
	if approvals.pool == nil {
		return ReviewPayload{}, ErrApprovalUnavailable
	}
	if !uuidPattern.MatchString(actionID) || !uuidPattern.MatchString(operator.UserID) || !uuidPattern.MatchString(operator.OrganizationID) {
		return ReviewPayload{}, ErrApprovalInvalid
	}
	var payload ReviewPayload
	err := pgx.BeginTxFunc(ctx, approvals.pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if err := requireReviewer(ctx, tx, operator); err != nil {
			return err
		}
		var raw []byte
		err := tx.QueryRow(ctx,
			`SELECT payload::text FROM runtime.review_payloads WHERE action_id = $1 AND organization_id = $2`,
			actionID, operator.OrganizationID).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrApprovalNotFound
		}
		if err != nil {
			return ErrApprovalUnavailable
		}
		if json.Unmarshal(raw, &payload) != nil {
			return ErrApprovalUnavailable
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNotReviewer) || errors.Is(err, ErrApprovalNotFound) {
			return ReviewPayload{}, err
		}
		return ReviewPayload{}, ErrApprovalUnavailable
	}
	return payload, nil
}
