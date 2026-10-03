package agent

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/contracts"
)

// Context entry kinds (runtime.context_entries.kind).
const (
	entryAssistantCall = "assistant_call"
	entryToolResult    = "tool_result"
	entryCorrection    = "correction"
)

// ErrContextStorage hides driver errors of the context store.
var ErrContextStorage = errors.New("agent context storage unavailable")

// ContextEntry is one stored piece of the model-visible working context.
type ContextEntry struct {
	StepNumber int
	Kind       string
	ActionID   string
	// Content is exactly what the model saw: the call, or the inspected result.
	Content           json.RawMessage
	InspectionOutcome string
	ReasonCode        string
}

// ContextStore appends and reads runtime.context_entries. Rows are never updated or deleted, so a
// restarted worker rebuilds the same model request and never re-executes a completed action.
type ContextStore struct{ pool *pgxpool.Pool }

// NewContextStore returns a store over the shared gateway pool.
func NewContextStore(pool *pgxpool.Pool) *ContextStore { return &ContextStore{pool: pool} }

// List returns the run's entries in step order, each call before its result.
func (store *ContextStore) List(ctx context.Context, organizationID, runID string) ([]ContextEntry, error) {
	if store == nil || store.pool == nil {
		return nil, ErrContextStorage
	}
	rows, err := store.pool.Query(ctx, `
		SELECT step_number, kind, COALESCE(action_id::text, ''), content,
		       COALESCE(inspection_outcome, ''), COALESCE(reason_code, '')
		FROM runtime.context_entries
		WHERE organization_id = $1 AND run_id = $2
		ORDER BY step_number, id`, organizationID, runID)
	if err != nil {
		return nil, ErrContextStorage
	}
	defer rows.Close()
	var entries []ContextEntry
	for rows.Next() {
		var entry ContextEntry
		if err = rows.Scan(&entry.StepNumber, &entry.Kind, &entry.ActionID, &entry.Content, &entry.InspectionOutcome, &entry.ReasonCode); err != nil {
			return nil, ErrContextStorage
		}
		entries = append(entries, entry)
	}
	if rows.Err() != nil {
		return nil, ErrContextStorage
	}
	return entries, nil
}

// AppendToolStep stores one executed step, the call and its inspected result, in one transaction.
func (store *ContextStore) AppendToolStep(ctx context.Context, organizationID, runID string, stepNumber int, actionID string,
	call json.RawMessage, result Inspection) error {
	if store == nil || store.pool == nil {
		return ErrContextStorage
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return ErrContextStorage
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var reason *string
	if result.Reason != "" {
		reasonText := string(result.Reason)
		reason = &reasonText
	}
	if _, err = transaction.Exec(ctx, `
		INSERT INTO runtime.context_entries(organization_id, run_id, step_number, kind, action_id, content)
		VALUES ($1, $2, $3, 'assistant_call', $4, $5)`, organizationID, runID, stepNumber, actionID, call); err != nil {
		return ErrContextStorage
	}
	if _, err = transaction.Exec(ctx, `
		INSERT INTO runtime.context_entries(organization_id, run_id, step_number, kind, action_id, content, inspection_outcome, reason_code)
		VALUES ($1, $2, $3, 'tool_result', $4, $5, $6, $7)`,
		organizationID, runID, stepNumber, actionID, result.Content, string(result.Outcome), reason); err != nil {
		return ErrContextStorage
	}
	if err = transaction.Commit(ctx); err != nil {
		return ErrContextStorage
	}
	return nil
}

// AppendCorrection stores a denied step in one transaction: the denied call when there was a
// single one (call may be nil) and the denial feedback the model receives next.
func (store *ContextStore) AppendCorrection(ctx context.Context, organizationID, runID string, stepNumber int,
	call json.RawMessage, feedback json.RawMessage, reason contracts.ReasonCode) error {
	if store == nil || store.pool == nil {
		return ErrContextStorage
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return ErrContextStorage
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if call != nil {
		if _, err = transaction.Exec(ctx, `
			INSERT INTO runtime.context_entries(organization_id, run_id, step_number, kind, content)
			VALUES ($1, $2, $3, 'assistant_call', $4)`, organizationID, runID, stepNumber, call); err != nil {
			return ErrContextStorage
		}
	}
	// A denied action may have no stored row (malformed arguments), so these rows carry no action
	// reference; the denial event links the action where one exists.
	if _, err = transaction.Exec(ctx, `
		INSERT INTO runtime.context_entries(organization_id, run_id, step_number, kind, content, reason_code)
		VALUES ($1, $2, $3, 'correction', $4, $5)`, organizationID, runID, stepNumber, feedback, string(reason)); err != nil {
		return ErrContextStorage
	}
	if err = transaction.Commit(ctx); err != nil {
		return ErrContextStorage
	}
	return nil
}
