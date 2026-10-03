package agent

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/policy"
)

// ErrRecovery hides driver errors of the recovery reads.
var ErrRecovery = errors.New("claim recovery unavailable")

// Recovery reconciles what a former claim of a run may have left behind before a new claim takes a
// step (GO-49, implementing GO-02). It never repeats a dispatch and never re-executes an action.
type Recovery struct {
	pool    *pgxpool.Pool
	ledger  budget.Store
	callLog *budget.CallLog
}

// NewRecovery returns the recovery over the shared gateway pool and the run ledger.
func NewRecovery(pool *pgxpool.Pool, ledger budget.Store) *Recovery {
	return &Recovery{pool: pool, ledger: ledger, callLog: budget.NewCallLog(pool)}
}

// openCall is a dispatch record without an outcome.
type openCall struct{ id string }

// orphanedStep is an executed action whose step never reached the context.
type orphanedStep struct {
	actionID           string
	stepNumber         int
	tool               string
	canonicalArguments json.RawMessage
}

// recoverClaim runs at the start of every claim:
//   - a model call without an outcome (the former claim stopped after its dispatch record or its
//     reservation) keeps its reservation as usage_unknown and pauses the run with outcome_unknown;
//     nothing is resent and nothing is counted as zero;
//   - an action still executing with an open attempt (stopped after the tool dispatch) pauses the run
//     with outcome_unknown for attention; it is never run again;
//   - an executed action whose step has no context entries (stopped after the effect committed and
//     before the context append) gets its call and a withheld-result marker, so the model continues
//     without the action being executed again.
func (loop *Loop) recoverClaim(ctx context.Context, run policy.RunIdentity) (runEnd, error) {
	recovery := loop.dependencies.Recovery
	calls, err := recovery.openCalls(ctx, run)
	if err != nil {
		return runEnd{}, err
	}
	for _, call := range calls {
		// A call that never reserved used nothing; a reserved one keeps its whole reservation.
		outcome := budget.CallUsageUnknown
		if markErr := recovery.ledger.MarkUnknown(ctx, run.RunID, call.id); errors.Is(markErr, budget.ErrNotFound) {
			outcome = budget.CallFailed
		} else if markErr != nil {
			return runEnd{}, markErr
		}
		if recordErr := recovery.callLog.RecordOutcome(ctx, run.OrganizationID, call.id, outcome); recordErr != nil && !errors.Is(recordErr, budget.ErrConflict) {
			return runEnd{}, recordErr
		}
	}
	executing, err := recovery.executingActions(ctx, run)
	if err != nil {
		return runEnd{}, err
	}
	if len(calls) > 0 || executing > 0 {
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonOutcomeUnknown}, nil
	}
	orphans, err := recovery.orphanedSteps(ctx, run)
	if err != nil {
		return runEnd{}, err
	}
	for _, orphan := range orphans {
		call, err := json.Marshal(struct {
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}{Tool: orphan.tool, Arguments: orphan.canonicalArguments})
		if err != nil {
			return runEnd{}, err
		}
		withheld, err := json.Marshal(struct {
			Withheld   bool   `json:"withheld"`
			ReasonCode string `json:"reason_code"`
		}{Withheld: true, ReasonCode: string(contracts.ReasonOutcomeUnknown)})
		if err != nil {
			return runEnd{}, err
		}
		marker := Inspection{Outcome: InspectionBlocked, Content: withheld, Reason: contracts.ReasonOutcomeUnknown}
		if err = loop.dependencies.Contexts.AppendToolStep(ctx, run.OrganizationID, run.RunID, orphan.stepNumber, orphan.actionID,
			call, marker, 0, ""); err != nil {
			return runEnd{}, err
		}
	}
	return runEnd{}, nil
}

// openCalls lists the run's dispatch records without an outcome.
func (recovery *Recovery) openCalls(ctx context.Context, run policy.RunIdentity) ([]openCall, error) {
	rows, err := recovery.pool.Query(ctx, `SELECT id::text FROM runtime.model_calls
		WHERE organization_id = $1 AND run_id = $2 AND outcome IS NULL ORDER BY dispatch_recorded_at`, run.OrganizationID, run.RunID)
	if err != nil {
		return nil, ErrRecovery
	}
	defer rows.Close()
	var calls []openCall
	for rows.Next() {
		var call openCall
		if rows.Scan(&call.id) != nil {
			return nil, ErrRecovery
		}
		calls = append(calls, call)
	}
	if rows.Err() != nil {
		return nil, ErrRecovery
	}
	return calls, nil
}

// executingActions counts the run's actions still executing with an attempt that has no outcome.
func (recovery *Recovery) executingActions(ctx context.Context, run policy.RunIdentity) (int, error) {
	var count int
	err := recovery.pool.QueryRow(ctx, `SELECT count(*) FROM runtime.actions AS action
		WHERE action.organization_id = $1 AND action.run_id = $2 AND action.status = 'executing'
		  AND EXISTS (SELECT 1 FROM runtime.execution_attempts AS attempt
		              WHERE attempt.action_id = action.id AND attempt.organization_id = action.organization_id
		                AND attempt.outcome IS NULL)`, run.OrganizationID, run.RunID).Scan(&count)
	if err != nil {
		return 0, ErrRecovery
	}
	return count, nil
}

// orphanedSteps lists the run's executed actions whose step has no context entries. Both the
// executor's current 'executed' and X-09's 'succeeded' status match, so either side can change alone.
func (recovery *Recovery) orphanedSteps(ctx context.Context, run policy.RunIdentity) ([]orphanedStep, error) {
	rows, err := recovery.pool.Query(ctx, `SELECT action.id::text, action.step_number, action.tool, action.canonical_arguments
		FROM runtime.actions AS action
		WHERE action.organization_id = $1 AND action.run_id = $2 AND action.status IN ('executed', 'succeeded')
		  AND NOT EXISTS (SELECT 1 FROM runtime.context_entries AS entry
		                  WHERE entry.organization_id = action.organization_id AND entry.run_id = action.run_id
		                    AND entry.step_number = action.step_number)
		ORDER BY action.step_number`, run.OrganizationID, run.RunID)
	if err != nil {
		return nil, ErrRecovery
	}
	defer rows.Close()
	var orphans []orphanedStep
	for rows.Next() {
		var orphan orphanedStep
		if rows.Scan(&orphan.actionID, &orphan.stepNumber, &orphan.tool, &orphan.canonicalArguments) != nil {
			return nil, ErrRecovery
		}
		orphans = append(orphans, orphan)
	}
	if rows.Err() != nil {
		return nil, ErrRecovery
	}
	return orphans, nil
}

// recoveryTimeout bounds one recovery pass.
const recoveryTimeout = 10 * time.Second
