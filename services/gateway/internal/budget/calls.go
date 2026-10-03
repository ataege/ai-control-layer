package budget

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CallOutcome is the recorded result of one model dispatch (runtime.model_calls.outcome).
type CallOutcome string

const (
	// CallCompleted: the provider answered with usable, settled usage.
	CallCompleted CallOutcome = "completed"
	// CallUsageUnknown: the request may have reached the provider, but usage is unknown
	// (timeout, transport error, missing counters); its reservation stays held.
	CallUsageUnknown CallOutcome = "usage_unknown"
	// CallFailed: the call failed before or without consuming provider work that Go can count,
	// or after a settled response was unusable.
	CallFailed CallOutcome = "failed"
)

// CallLog writes the GO-02 pre-dispatch record of every model call. The record is committed
// before the request is sent, so it proves intent to dispatch, not delivery.
type CallLog struct{ pool *pgxpool.Pool }

// NewCallLog returns a call log over the shared gateway pool.
func NewCallLog(pool *pgxpool.Pool) *CallLog { return &CallLog{pool: pool} }

// RecordDispatch commits the dispatch record and returns its id, which is also the ledger's
// call id (alignment decision 3).
func (callLog *CallLog) RecordDispatch(ctx context.Context, organizationID, runID, purpose, model string) (string, error) {
	if callLog == nil || callLog.pool == nil {
		return "", ErrUnavailable
	}
	if organizationID == "" || runID == "" || model == "" || (purpose != "agent" && purpose != "security") {
		return "", ErrInvalid
	}
	var callID string
	err := callLog.pool.QueryRow(ctx, `
		INSERT INTO runtime.model_calls(organization_id, run_id, purpose, model)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text`, organizationID, runID, purpose, model).Scan(&callID)
	if err != nil {
		return "", ErrUnavailable
	}
	return callID, nil
}

// RecordOutcome sets the outcome of a dispatched call once; a second outcome is rejected.
func (callLog *CallLog) RecordOutcome(ctx context.Context, organizationID, callID string, outcome CallOutcome) error {
	if callLog == nil || callLog.pool == nil {
		return ErrUnavailable
	}
	if organizationID == "" || callID == "" || (outcome != CallCompleted && outcome != CallUsageUnknown && outcome != CallFailed) {
		return ErrInvalid
	}
	tag, err := callLog.pool.Exec(ctx, `
		UPDATE runtime.model_calls SET outcome = $3, completed_at = now()
		WHERE id = $1 AND organization_id = $2 AND outcome IS NULL`, callID, organizationID, string(outcome))
	if err != nil {
		return ErrUnavailable
	}
	if tag.RowsAffected() != 1 {
		return errors.Join(ErrConflict, errors.New("model call outcome already recorded or call not found"))
	}
	return nil
}

// CountAgentCalls returns how many agent-purpose calls the run has dispatched. The agent loop uses
// it as the run's step count: one agent call per step, whatever the step produced.
func (callLog *CallLog) CountAgentCalls(ctx context.Context, organizationID, runID string) (int, error) {
	if callLog == nil || callLog.pool == nil {
		return 0, ErrUnavailable
	}
	if organizationID == "" || runID == "" {
		return 0, ErrInvalid
	}
	var count int
	err := callLog.pool.QueryRow(ctx, `
		SELECT count(*) FROM runtime.model_calls
		WHERE organization_id = $1 AND run_id = $2 AND purpose = 'agent'`, organizationID, runID).Scan(&count)
	if err != nil {
		return 0, ErrUnavailable
	}
	return count, nil
}
