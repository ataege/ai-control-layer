// Package budget persists shared model allowances before requests are dispatched.
package budget

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid     = errors.New("invalid model budget operation")
	ErrNotFound    = errors.New("model budget or reservation not found")
	ErrExhausted   = errors.New("model token budget exhausted")
	ErrPaused      = errors.New("model token budget paused")
	ErrDuplicate   = errors.New("model call already reserved")
	ErrConflict    = errors.New("model usage conflicts with settled usage")
	ErrUnavailable = errors.New("model budget storage unavailable")
)

type Reservation struct{ Tokens int64 }
type Snapshot struct {
	Limit, Reserved, Used int64
	Paused                bool
}
type Settlement struct {
	ActualTokens, RefundedTokens int64
	Paused, AlreadySettled       bool
}
type Store interface {
	Reserve(context.Context, string, string, string, int64) (Reservation, error)
	MarkUnknown(context.Context, string, string) error
	Settle(context.Context, string, string, int64, int64) (Settlement, error)
	Snapshot(context.Context, string) (Snapshot, error)
}
type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }
func (s *PostgresStore) CreateRun(ctx context.Context, runID string, limit int64) error {
	if ctx == nil || runID == "" || limit <= 0 {
		return ErrInvalid
	}
	if s.pool == nil {
		return ErrUnavailable
	}
	_, err := s.pool.Exec(ctx, "INSERT INTO runtime.model_token_budgets(run_id,token_limit) VALUES($1,$2)", runID, limit)
	if err != nil {
		return ErrUnavailable
	}
	return nil
}
func storageError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return ErrUnavailable
}
func (s *PostgresStore) Snapshot(ctx context.Context, runID string) (Snapshot, error) {
	var b Snapshot
	if ctx == nil || runID == "" {
		return b, ErrInvalid
	}
	if s.pool == nil {
		return b, ErrUnavailable
	}
	err := s.pool.QueryRow(ctx, "SELECT token_limit,reserved_tokens,used_tokens,paused FROM runtime.model_token_budgets WHERE run_id=$1", runID).Scan(&b.Limit, &b.Reserved, &b.Used, &b.Paused)
	if err != nil {
		return b, storageError(err)
	}
	return b, nil
}
func (s *PostgresStore) lock(ctx context.Context, runID string) (pgx.Tx, Snapshot, error) {
	var b Snapshot
	if s.pool == nil {
		return nil, b, ErrUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, b, ErrUnavailable
	}
	err = tx.QueryRow(ctx, "SELECT token_limit,reserved_tokens,used_tokens,paused FROM runtime.model_token_budgets WHERE run_id=$1 FOR UPDATE", runID).Scan(&b.Limit, &b.Reserved, &b.Used, &b.Paused)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, b, storageError(err)
	}
	return tx, b, nil
}
func (s *PostgresStore) Reserve(ctx context.Context, runID, callID, purpose string, tokens int64) (Reservation, error) {
	var result Reservation
	if ctx == nil || runID == "" || callID == "" || (purpose != "agent" && purpose != "security") || tokens <= 0 {
		return result, ErrInvalid
	}
	tx, b, err := s.lock(ctx, runID)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM runtime.model_token_reservations WHERE run_id=$1 AND call_id=$2)", runID, callID).Scan(&exists)
	if err != nil {
		return result, ErrUnavailable
	}
	if exists {
		return result, ErrDuplicate
	}
	if b.Paused {
		return result, ErrPaused
	}
	if b.Used > b.Limit || b.Reserved > b.Limit-b.Used || tokens > b.Limit-b.Used-b.Reserved {
		return result, ErrExhausted
	}
	_, err = tx.Exec(ctx, "INSERT INTO runtime.model_token_reservations(run_id,call_id,purpose,token_reservation,status) VALUES($1,$2,$3,$4,'reserved')", runID, callID, purpose, tokens)
	if err != nil {
		return result, ErrUnavailable
	}
	_, err = tx.Exec(ctx, "UPDATE runtime.model_token_budgets SET reserved_tokens=reserved_tokens+$2 WHERE run_id=$1", runID, tokens)
	if err != nil {
		return result, ErrUnavailable
	}
	if tx.Commit(ctx) != nil {
		return result, ErrUnavailable
	}
	return Reservation{Tokens: tokens}, nil
}
func (s *PostgresStore) MarkUnknown(ctx context.Context, runID, callID string) error {
	if ctx == nil || runID == "" || callID == "" {
		return ErrInvalid
	}
	tx, _, err := s.lock(ctx, runID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "UPDATE runtime.model_token_reservations SET status=CASE WHEN status='settled' THEN status ELSE 'usage_unknown' END WHERE run_id=$1 AND call_id=$2", runID, callID)
	if err != nil {
		return ErrUnavailable
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	if tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *PostgresStore) Settle(ctx context.Context, runID, callID string, input, output int64) (Settlement, error) {
	var result Settlement
	if ctx == nil || runID == "" || callID == "" || input < 0 || output < 0 || input > math.MaxInt64-output {
		return result, ErrInvalid
	}
	actual := input + output
	tx, b, err := s.lock(ctx, runID)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	var reserved int64
	var status string
	var oldInput, oldOutput *int64
	err = tx.QueryRow(ctx, "SELECT token_reservation,status,input_tokens,output_tokens FROM runtime.model_token_reservations WHERE run_id=$1 AND call_id=$2 FOR UPDATE", runID, callID).Scan(&reserved, &status, &oldInput, &oldOutput)
	if err != nil {
		return result, storageError(err)
	}
	result.ActualTokens = actual
	result.Paused = b.Paused
	if actual < reserved {
		result.RefundedTokens = reserved - actual
	}
	if status == "settled" {
		if oldInput == nil || oldOutput == nil || *oldInput != input || *oldOutput != output {
			return Settlement{}, ErrConflict
		}
		result.AlreadySettled = true
		return result, nil
	}
	if reserved > b.Reserved {
		return Settlement{}, ErrInvalid
	}
	if actual > math.MaxInt64-b.Used {
		// The measured count cannot fit the aggregate. Fence dispatch durably while
		// retaining its reservation; never clip usage or release an unknown balance.
		if _, err = tx.Exec(ctx, "UPDATE runtime.model_token_budgets SET paused=true WHERE run_id=$1", runID); err != nil {
			return Settlement{}, ErrUnavailable
		}
		if _, err = tx.Exec(ctx, "UPDATE runtime.model_token_reservations SET status='usage_unknown' WHERE run_id=$1 AND call_id=$2", runID, callID); err != nil {
			return Settlement{}, ErrUnavailable
		}
		if tx.Commit(ctx) != nil {
			return Settlement{}, ErrUnavailable
		}
		return Settlement{ActualTokens: actual, Paused: true}, ErrInvalid
	}
	result.Paused = b.Paused || actual > reserved
	_, err = tx.Exec(ctx, "UPDATE runtime.model_token_budgets SET reserved_tokens=reserved_tokens-$2,used_tokens=used_tokens+$3,paused=$4 WHERE run_id=$1", runID, reserved, actual, result.Paused)
	if err != nil {
		return Settlement{}, ErrUnavailable
	}
	_, err = tx.Exec(ctx, "UPDATE runtime.model_token_reservations SET status='settled',input_tokens=$3,output_tokens=$4,actual_tokens=$5 WHERE run_id=$1 AND call_id=$2", runID, callID, input, output, actual)
	if err != nil {
		return Settlement{}, ErrUnavailable
	}
	if tx.Commit(ctx) != nil {
		return Settlement{}, ErrUnavailable
	}
	return result, nil
}
