package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/provenance"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/tools"
)

// Action status values the executor writes after the gate's decision.
const (
	actionStatusExecuting = "executing"
	actionStatusExecuted  = string(contracts.ActionSucceeded) // X-09 "succeeded"
	actionStatusFailed    = "failed"
)

// Attempt outcome for an attempt whose transaction was rolled back before commit: no effect was
// committed, so the outcome is known.
const attemptOutcomeAborted = "aborted"

// ExecutionStatus says what happened to one execution request.
type ExecutionStatus string

const (
	// ExecutionSucceeded: the adapter ran and its effect, attempt completion and event committed.
	ExecutionSucceeded ExecutionStatus = "succeeded"
	// ExecutionFailed: the adapter refused with a known reason; nothing was changed but the record.
	ExecutionFailed ExecutionStatus = "failed"
	// ExecutionRefused: a fresh check failed before dispatch; no adapter was called.
	ExecutionRefused ExecutionStatus = "refused"
	// ExecutionPaused: a storage problem or an unknown outcome. The run pauses; nothing is
	// retried blindly.
	ExecutionPaused ExecutionStatus = "paused"
	// ExecutionStopped: the effect's preconditions no longer hold (the stored action, attempt or
	// passport). The action is failed; the worker stops the run with the reason. No retry.
	ExecutionStopped ExecutionStatus = "stopped"
)

// ExecutionResult is what the worker receives. Result is the minimized form (GO-23) whose
// untrusted text still has to pass the tool-result inspection (GO-76) before it becomes model
// context; the raw adapter result never leaves the executor.
type ExecutionResult struct {
	Status     ExecutionStatus
	ReasonCode ReasonCode
	AttemptID  string
	Result     tools.MinimizedResult
}

// Executor dispatches an allowed action to its registered adapter by its stable identifier.
type Executor struct {
	pool   *pgxpool.Pool
	scopes ScopeReader
	runner tools.EffectRunner
	// beforeCommit runs just before the effect's commit; tests use it to make the commit fail.
	beforeCommit func()
}

// NewExecutor returns an executor. It creates nothing at construction.
func NewExecutor(pool *pgxpool.Pool, scopes ScopeReader, runner tools.EffectRunner) *Executor {
	return &Executor{pool: pool, scopes: scopes, runner: runner}
}

// storedExecutableAction is the stored action plus the run facts the fresh checks need.
type storedExecutableAction struct {
	tool                ToolName
	canonicalArguments  []byte
	actionDigest        []byte
	evaluatedRevisionID int64
	status              string
	passportID          string
	runActive           bool
	passportUnexpired   bool
	replaySource        *string // labelled replay (GO-36); events read it from the row
}

// maximumSafeRetries bounds the retries of a known no-effect failure within one Execute call;
// every retry is a new attempt and counts against the passport's attempt limit.
const maximumSafeRetries = 1

// executionRetry is the internal signal of a rolled-back, retry-safe failure (GO-53).
const executionRetry ExecutionStatus = "retry"

// Execute runs one allowed or approved action: fresh checks, then a committed attempt record,
// then the adapter, the attempt completion and its event in one transaction. A known no-effect
// failure of a retry-safe tool is retried under the same action id (GO-53).
func (executor *Executor) Execute(ctx context.Context, run RunIdentity, actionID string) ExecutionResult {
	result := executor.executeOnce(ctx, run, actionID)
	for retries := 0; result.Status == executionRetry; retries++ {
		if retries >= maximumSafeRetries {
			return ExecutionResult{Status: ExecutionPaused, ReasonCode: ReasonDecisionUnavailable, AttemptID: result.AttemptID}
		}
		result = executor.executeOnce(ctx, run, actionID)
	}
	return result
}

func (executor *Executor) executeOnce(ctx context.Context, run RunIdentity, actionID string) ExecutionResult {
	refused := func(reason ReasonCode) ExecutionResult {
		return ExecutionResult{Status: ExecutionRefused, ReasonCode: reason}
	}
	paused := func(reason ReasonCode) ExecutionResult {
		return ExecutionResult{Status: ExecutionPaused, ReasonCode: reason}
	}
	if executor.pool == nil || executor.runner == nil || executor.scopes == nil {
		return paused(ReasonDecisionUnavailable)
	}

	action, err := executor.loadAction(ctx, run, actionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return refused(ReasonActionChanged)
	}
	if err != nil {
		return paused(ReasonDecisionUnavailable)
	}
	// Only an allowed action, or an approved one (GO-45), not yet executed, runs.
	approved := action.status == string(contracts.ActionApproved)
	if action.status != actionStatusAllowed && !approved {
		return refused(ReasonActionChanged)
	}
	// A cancelled or stopped run and an expired passport each stop dispatch with their own reason.
	if !action.runActive {
		return refused(ReasonRunCancelled)
	}
	if !action.passportUnexpired {
		return refused(ReasonRunExpired)
	}
	if !digestStillMatches(run, actionID, action) {
		return refused(ReasonActionChanged)
	}

	scope, err := executor.scopes.LoadScope(ctx, run)
	if err != nil || scope.PassportID != action.passportID {
		return paused(ReasonDecisionUnavailable)
	}
	// The action was evaluated under one catalog revision; a different active revision may carry
	// stricter rules, so the action needs a fresh evaluation instead of running on a stale one.
	activeRevision, err := executor.scopes.ActiveCatalogRevision(ctx)
	if err != nil {
		return paused(ReasonDecisionUnavailable)
	}
	if activeRevision != action.evaluatedRevisionID {
		return refused(contracts.ReasonSourcePolicyChanged)
	}
	if approved {
		reason, err := executor.recheckApproved(ctx, run, actionID, action)
		if err != nil {
			return paused(ReasonDecisionUnavailable)
		}
		if reason != "" {
			return refused(reason)
		}
	}
	attemptID, err := executor.recordAttempt(ctx, run, actionID, scope.ToolAttemptLimit)
	if errors.Is(err, errAttemptLimitReached) {
		return refused(ReasonAllowanceExhausted)
	}
	if errors.Is(err, errActionNotExecutable) {
		return refused(ReasonActionChanged)
	}
	if err != nil {
		return paused(ReasonDecisionUnavailable)
	}

	var digest [32]byte
	copy(digest[:], action.actionDigest)
	request := tools.EffectRequest{
		OrganizationID: run.OrganizationID, RunID: run.RunID, PassportID: action.passportID,
		ActionID: actionID, AttemptID: attemptID, Tool: string(action.tool),
		CanonicalArguments: action.canonicalArguments, ActionDigest: digest,
		CatalogRevisionID: action.evaluatedRevisionID,
	}
	return executor.runEffect(ctx, run, request, approved)
}

// runEffect calls the adapter in one transaction and commits it. An adapter error rolls back,
// so no effect was committed and the attempt is closed as aborted; a failed commit leaves the
// outcome unknown and the attempt open.
func (executor *Executor) runEffect(ctx context.Context, run RunIdentity, request tools.EffectRequest, consumeApproval bool) ExecutionResult {
	tx, err := executor.pool.Begin(ctx)
	if err != nil {
		executor.abortAttempt(ctx, run, request)
		return ExecutionResult{Status: ExecutionPaused, ReasonCode: ReasonDecisionUnavailable, AttemptID: request.AttemptID}
	}
	if consumeApproval {
		// The bound grant is consumed once, by this attempt, in the effect's transaction: a
		// consumed, rejected or expired grant leaves nothing to consume and nothing runs.
		consumed, err := consumeApprovalGrant(ctx, tx, run, request)
		if err != nil || !consumed {
			_ = tx.Rollback(ctx)
			executor.abortAttempt(ctx, run, request)
			if err != nil {
				return ExecutionResult{Status: ExecutionPaused, ReasonCode: ReasonDecisionUnavailable, AttemptID: request.AttemptID}
			}
			return ExecutionResult{Status: ExecutionRefused, ReasonCode: contracts.ReasonApprovalExpired, AttemptID: request.AttemptID}
		}
	}
	effect, runErr := executor.runner.RunEffect(ctx, tx, request)
	statusErr := error(nil)
	if runErr == nil {
		statusErr = setExecutedStatus(ctx, tx, run, request.ActionID, effect.Outcome)
	}
	if runErr != nil || statusErr != nil {
		// Rolled back: every demo effect is local to this transaction, so nothing committed.
		_ = tx.Rollback(ctx)
		executor.abortAttempt(ctx, run, request)
		failure := tools.ClassifyRunError(runErr)
		switch {
		case (runErr != nil && tools.RetrySafe(request.Tool, failure)) || (statusErr != nil && !errors.Is(statusErr, errActionNotExecutable)):
			// Back to allowed or approved, so the retry claims the action again with fresh checks;
			// a consumed grant was rolled back with the effect and is reused once.
			if executor.releaseAction(ctx, run, request.ActionID, consumeApproval) != nil {
				return ExecutionResult{Status: ExecutionPaused, ReasonCode: ReasonDecisionUnavailable, AttemptID: request.AttemptID}
			}
			return ExecutionResult{Status: executionRetry, AttemptID: request.AttemptID}
		case runErr != nil && failure == tools.FailurePrecondition, errors.Is(statusErr, errActionNotExecutable):
			executor.failAction(ctx, run, request)
			return ExecutionResult{Status: ExecutionStopped, ReasonCode: ReasonActionChanged, AttemptID: request.AttemptID}
		}
		return ExecutionResult{Status: ExecutionPaused, ReasonCode: ReasonDecisionUnavailable, AttemptID: request.AttemptID}
	}
	if executor.beforeCommit != nil {
		executor.beforeCommit()
	}
	if err := tx.Commit(ctx); err != nil {
		// The commit may or may not have reached the database. Establish the outcome from the
		// stored status; when it cannot be established, record it as unknown and never retry.
		if executor.commitLanded(ctx, run, request.ActionID) {
			return executor.finishCommitted(request, effect)
		}
		executor.recordUnknownOutcome(ctx, run, request)
		return ExecutionResult{Status: ExecutionPaused, ReasonCode: ReasonOutcomeUnknown, AttemptID: request.AttemptID}
	}
	return executor.finishCommitted(request, effect)
}

// finishCommitted turns a committed effect into the worker's minimized result.
func (executor *Executor) finishCommitted(request tools.EffectRequest, effect tools.EffectResult) ExecutionResult {
	outcome := ExecutionResult{AttemptID: request.AttemptID}

	minimized, err := tools.MinimizeForModel(request.Tool, effect)
	if err != nil {
		// The effect is committed; only its model-facing form is unavailable.
		return ExecutionResult{Status: ExecutionPaused, ReasonCode: ReasonDecisionUnavailable, AttemptID: request.AttemptID}
	}
	outcome.Result = minimized
	if effect.Outcome == tools.OutcomeSucceeded {
		outcome.Status = ExecutionSucceeded
	} else {
		outcome.Status, outcome.ReasonCode = ExecutionFailed, ReasonCode(effect.ReasonCode)
	}
	return outcome
}

func (executor *Executor) loadAction(ctx context.Context, run RunIdentity, actionID string) (storedExecutableAction, error) {
	var action storedExecutableAction
	var tool string
	err := executor.pool.QueryRow(ctx,
		`SELECT a.tool, a.canonical_arguments::text, a.action_digest, a.evaluated_catalog_revision_id, a.status,
		        r.passport_id::text,
		        (r.cancel_requested_at IS NULL AND r.status = $4),
		        p.expires_at > now(), a.replay_source
		   FROM runtime.actions a
		   JOIN runtime.runs r ON r.id = a.run_id AND r.organization_id = a.organization_id
		   JOIN runtime.passports p ON p.id = r.passport_id AND p.organization_id = r.organization_id
		  WHERE a.id = $1 AND a.organization_id = $2 AND a.run_id = $3`,
		actionID, run.OrganizationID, run.RunID, string(contracts.RunRunning),
	).Scan(&tool, &action.canonicalArguments, &action.actionDigest, &action.evaluatedRevisionID, &action.status,
		&action.passportID, &action.runActive, &action.passportUnexpired, &action.replaySource)
	action.tool = ToolName(tool)
	return action, err
}

// digestStillMatches decodes the stored arguments again and recomputes the digest the gate stored
// (stored jsonb reorders keys, so the stored bytes are never hashed directly).
func digestStillMatches(run RunIdentity, actionID string, action storedExecutableAction) bool {
	arguments, err := DecodeArguments(action.tool, action.canonicalArguments)
	if err != nil {
		return false
	}
	recomputed, err := CanonicalAction{
		ActionID: actionID, RunID: run.RunID, Arguments: arguments,
		PassportID: action.passportID, PolicyRevisionID: action.evaluatedRevisionID,
	}.Digest()
	return err == nil && bytes.Equal(recomputed[:], action.actionDigest)
}

var (
	errAttemptLimitReached = errors.New("tool attempt limit reached")
	// errActionNotExecutable means another execution of the same action got there first.
	errActionNotExecutable = errors.New("action is no longer executable")
)

// recordAttempt claims the action, counts the run's attempts against the passport's limit, then
// inserts and commits the open attempt (GO-02: the record of intent exists before dispatch).
// Lock order matters: the action is claimed first (allowed or approved -> executing), so a
// concurrent execution of the same action finds nothing to claim and returns at once, holding no
// lock. Only then is the run row locked FOR NO KEY UPDATE, which serializes the attempt count
// across the run's actions; it is the same lock the event writer takes, so a waiting attempt
// insert never holds it while a running effect needs it.
func (executor *Executor) recordAttempt(ctx context.Context, run RunIdentity, actionID string, attemptLimit int) (string, error) {
	var attemptID string
	err := pgx.BeginFunc(ctx, executor.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE runtime.actions SET status = $1, updated_at = now()
			  WHERE id = $2 AND organization_id = $3 AND status IN ($4, $5)`,
			actionStatusExecuting, actionID, run.OrganizationID, actionStatusAllowed, string(contracts.ActionApproved))
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errActionNotExecutable // a concurrent execution claimed it first
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM runtime.runs WHERE id = $1 AND organization_id = $2 FOR NO KEY UPDATE`,
			run.RunID, run.OrganizationID); err != nil {
			return err
		}
		var usedAttempts int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM runtime.execution_attempts e
			   JOIN runtime.actions a ON a.id = e.action_id AND a.organization_id = e.organization_id
			  WHERE a.run_id = $1 AND a.organization_id = $2`,
			run.RunID, run.OrganizationID).Scan(&usedAttempts); err != nil {
			return err
		}
		if usedAttempts >= attemptLimit {
			return errAttemptLimitReached // the rollback also returns the action to its status
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO runtime.execution_attempts (organization_id, action_id, attempt_number)
			 SELECT $1, $2, coalesce(max(attempt_number), 0) + 1
			   FROM runtime.execution_attempts WHERE action_id = $2 AND organization_id = $1
			 RETURNING id::text`,
			run.OrganizationID, actionID).Scan(&attemptID); err != nil {
			var postgresError *pgconn.PgError
			if errors.As(err, &postgresError) && postgresError.Code == "23505" {
				return errActionNotExecutable // another attempt of this action is open
			}
			return err
		}
		return nil
	})
	return attemptID, err
}

// setExecutedStatus moves the action to its final status inside the effect's transaction.
func setExecutedStatus(ctx context.Context, tx pgx.Tx, run RunIdentity, actionID, effectOutcome string) error {
	finalStatus := actionStatusFailed
	if effectOutcome == tools.OutcomeSucceeded {
		finalStatus = actionStatusExecuted
	}
	tag, err := tx.Exec(ctx,
		`UPDATE runtime.actions SET status = $1, updated_at = now()
		  WHERE id = $2 AND organization_id = $3 AND run_id = $4 AND status = $5`,
		finalStatus, actionID, run.OrganizationID, run.RunID, actionStatusExecuting)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errActionNotExecutable // the action is no longer executing
	}
	return nil
}

// abortAttempt closes an attempt whose transaction rolled back. If even that fails, the attempt
// stays open and blocks a second attempt for the action until it is reconciled.
func (executor *Executor) abortAttempt(ctx context.Context, run RunIdentity, request tools.EffectRequest) {
	_, _ = executor.pool.Exec(ctx,
		`UPDATE runtime.execution_attempts SET outcome = $1, completed_at = now()
		  WHERE id = $2 AND organization_id = $3 AND completed_at IS NULL`,
		attemptOutcomeAborted, request.AttemptID, run.OrganizationID)
}

// recheckApproved runs the GO-45 rechecks of an approved action before anything is written: the
// grant is still open and unexpired, and the review material rebuilt from current rows (recipient
// address, report content and lineage versions, template) still has the frozen digest. A changed
// source version is resource_version_changed; any other change, or a recipient or report that no
// longer resolves, is action_changed.
func (executor *Executor) recheckApproved(ctx context.Context, run RunIdentity, actionID string, action storedExecutableAction) (ReasonCode, error) {
	var reason ReasonCode
	err := pgx.BeginTxFunc(ctx, executor.pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var grantOpen bool
		err := tx.QueryRow(ctx,
			`SELECT decision = 'approved' AND consumed_at IS NULL AND expires_at > now()
			   FROM runtime.approvals WHERE action_id = $1 AND organization_id = $2`,
			actionID, run.OrganizationID).Scan(&grantOpen)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !grantOpen) {
			reason = contracts.ReasonApprovalExpired
			return nil
		}
		if err != nil {
			return err
		}
		var rawPayload, frozenDigest []byte
		if err := tx.QueryRow(ctx,
			`SELECT payload::text, payload_digest FROM runtime.review_payloads WHERE action_id = $1 AND organization_id = $2`,
			actionID, run.OrganizationID).Scan(&rawPayload, &frozenDigest); err != nil {
			reason = ReasonActionChanged
			return nil
		}
		var frozen ReviewPayload
		if json.Unmarshal(rawPayload, &frozen) != nil {
			reason = ReasonActionChanged
			return nil
		}
		current, _, err := buildReviewPayload(ctx, tx, run, StoredAction{
			ActionID: actionID, Tool: action.tool, CanonicalArguments: action.canonicalArguments,
			EvaluatedRevisionID: action.evaluatedRevisionID,
		}, action.passportID, frozen.ExpiresAt)
		if err != nil {
			reason = ReasonActionChanged
			return nil
		}
		// The frozen and the rebuilt payload both carry the lineage's versions, so the current
		// versions of the sources are compared explicitly (exact integer equality).
		if frozen.Report != nil {
			var sourceIDs []string
			for _, source := range frozen.Report.Sources {
				sourceIDs = append(sourceIDs, source.ID)
			}
			versions, err := provenance.CurrentInvoiceVersions(ctx, tx, run.OrganizationID, sourceIDs)
			if err != nil {
				return err
			}
			if len(StaleSources(frozen, versions)) > 0 {
				reason = contracts.ReasonResourceVersionChanged
				return nil
			}
		}
		currentDigest, _, err := current.Digest()
		if err != nil || !bytes.Equal(currentDigest[:], frozenDigest) {
			reason = ReasonActionChanged
		}
		return nil
	})
	return reason, err
}

// consumeApprovalGrant marks the action's approved grant consumed by this attempt. The database
// guard allows one consumption, before expiry, of an approved grant only.
func consumeApprovalGrant(ctx context.Context, tx pgx.Tx, run RunIdentity, request tools.EffectRequest) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE runtime.approvals SET consumed_at = now(), consumed_by_attempt_id = $3
		  WHERE action_id = $1 AND organization_id = $2 AND decision = 'approved'
		    AND consumed_at IS NULL AND expires_at > now()`,
		request.ActionID, run.OrganizationID, request.AttemptID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// releaseAction returns an action from executing to allowed or approved after a rolled-back,
// retry-safe failure, so the retry claims it again through the normal checks.
func (executor *Executor) releaseAction(ctx context.Context, run RunIdentity, actionID string, approved bool) error {
	status := actionStatusAllowed
	if approved {
		status = string(contracts.ActionApproved)
	}
	tag, err := executor.pool.Exec(ctx,
		`UPDATE runtime.actions SET status = $1, updated_at = now()
		  WHERE id = $2 AND organization_id = $3 AND status = $4`,
		status, actionID, run.OrganizationID, actionStatusExecuting)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errActionNotExecutable
	}
	return nil
}

// failAction marks an action whose effect preconditions no longer hold as failed, with its
// action.failed event. A failure to record it leaves the action executing, which blocks it.
func (executor *Executor) failAction(ctx context.Context, run RunIdentity, request tools.EffectRequest) {
	recordContext := context.WithoutCancel(ctx)
	_ = repository.New(executor.pool).InTransaction(recordContext, func(tx repository.Tx) error {
		tag, err := tx.Raw().Exec(recordContext,
			`UPDATE runtime.actions SET status = $1, updated_at = now()
			  WHERE id = $2 AND organization_id = $3 AND status = $4`,
			actionStatusFailed, request.ActionID, run.OrganizationID, actionStatusExecuting)
		if err != nil || tag.RowsAffected() != 1 {
			return errActionNotExecutable
		}
		return appendActionEvent(recordContext, tx, run, request, contracts.EventActionFailed, ReasonActionChanged)
	})
}

// commitLanded reads the action's status after a failed commit: executed or failed means the
// effect's transaction reached the database.
func (executor *Executor) commitLanded(ctx context.Context, run RunIdentity, actionID string) bool {
	var status string
	err := executor.pool.QueryRow(context.WithoutCancel(ctx),
		`SELECT status FROM runtime.actions WHERE id = $1 AND organization_id = $2`, actionID, run.OrganizationID).Scan(&status)
	return err == nil && (status == actionStatusExecuted || status == actionStatusFailed)
}

// recordUnknownOutcome keeps the attempt open (the one-open-attempt index then blocks any second
// attempt), sets the action to unknown and writes action.unknown with outcome_unknown. The run
// pauses for reconciliation through the worker; nothing re-queues the action.
func (executor *Executor) recordUnknownOutcome(ctx context.Context, run RunIdentity, request tools.EffectRequest) {
	recordContext := context.WithoutCancel(ctx)
	_ = repository.New(executor.pool).InTransaction(recordContext, func(tx repository.Tx) error {
		tag, err := tx.Raw().Exec(recordContext,
			`UPDATE runtime.actions SET status = $1, updated_at = now()
			  WHERE id = $2 AND organization_id = $3 AND status = $4`,
			string(contracts.ActionUnknown), request.ActionID, run.OrganizationID, actionStatusExecuting)
		if err != nil || tag.RowsAffected() != 1 {
			return errActionNotExecutable
		}
		return appendActionEvent(recordContext, tx, run, request, contracts.EventActionUnknown, ReasonOutcomeUnknown)
	})
}

// appendActionEvent writes one executor event about the action, references only. A replayed
// action's label is copied from the stored action, so every record of it carries it.
func appendActionEvent(ctx context.Context, tx repository.Tx, run RunIdentity, request tools.EffectRequest, eventType contracts.EventType, reason ReasonCode) error {
	var replaySource *string
	if err := tx.Raw().QueryRow(ctx, `SELECT replay_source FROM runtime.actions WHERE id = $1 AND organization_id = $2`,
		request.ActionID, run.OrganizationID).Scan(&replaySource); err != nil {
		return err
	}
	runID, actionID, revisionID := run.RunID, request.ActionID, request.CatalogRevisionID
	_, err := tx.AppendEvent(ctx, repository.NewEvent{
		OrganizationID: run.OrganizationID, RunID: &runID, ActionID: &actionID, EventType: eventType,
		ReasonCode: &reason, CatalogRevisionID: &revisionID,
		MaskedSummary: contracts.MaskedSummary{ReplaySource: replaySource},
	})
	return err
}
