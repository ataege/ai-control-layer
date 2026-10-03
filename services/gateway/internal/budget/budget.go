// Package budget is the run's model allowance ledger, the single authority for model calls,
// tokens (shared and per purpose), request time and the per-run concurrency slot (alignment
// decisions 1 to 7). Every balance mutation locks the run's ledger row first, in a short
// transaction that never spans a model request.
package budget

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid  = errors.New("invalid model budget operation")
	ErrNotFound = errors.New("model budget or reservation not found")
	// ErrExhausted covers every exhausted allowance: shared or purpose tokens and call counts. The
	// wrapped message names which; errors.Is(err, ErrExhausted) holds for all of them.
	ErrExhausted = errors.New("model allowance exhausted")
	ErrPaused    = errors.New("model token budget paused")
	ErrDuplicate = errors.New("model call already reserved")
	ErrConflict  = errors.New("model usage conflicts with settled usage")
	// ErrConcurrencyLimit means the run already holds all its concurrent call slots. Nothing is
	// reserved; the caller may try again later.
	ErrConcurrencyLimit = errors.New("model call concurrency limit reached")
	ErrUnavailable      = errors.New("model budget storage unavailable")
)

// Model purposes, assigned by trusted runtime code.
const (
	PurposeAgent    = "agent"
	PurposeSecurity = "security"
)

// Reservation is what Reserve granted: the reserved tokens and the request time the caller must
// bound the provider request with.
type Reservation struct {
	Tokens         int64
	RequestTimeout time.Duration
}

// PurposeUsage is one purpose's share of the ledger.
type PurposeUsage struct {
	// TokenLimit is nil when the passport sets no sub-limit for the purpose.
	TokenLimit      *int64
	ReservedTokens  int64
	UsedTokens      int64
	CallLimit       int64
	Calls           int64
	UnresolvedCalls int64
}

// Snapshot is the run's ledger as stored. Unresolved reservations are reported, never zero.
type Snapshot struct {
	Limit, Reserved, Used int64
	Paused                bool
	CallLimit             int64
	Agent, Security       PurposeUsage
	MaxConcurrentCalls    int64
	CallsInFlight         int64
	RequestTimeout        time.Duration
}

// Settlement is the outcome of settling one call's usage.
type Settlement struct {
	ActualTokens, RefundedTokens int64
	Paused, AlreadySettled       bool
}

// Store is the ledger the accounted model gateway uses.
type Store interface {
	Reserve(context.Context, string, string, string, int64) (Reservation, error)
	MarkUnknown(context.Context, string, string) error
	Settle(context.Context, string, string, int64, int64) (Settlement, error)
	Snapshot(context.Context, string) (Snapshot, error)
}

// PostgresStore keeps the ledger in runtime.model_token_budgets and model_token_reservations.
type PostgresStore struct{ pool *pgxpool.Pool }

// NewPostgresStore returns a ledger over the shared gateway pool. It creates nothing.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func storageError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return ErrUnavailable
}

// ledgerRow is the locked balance of one run.
type ledgerRow struct {
	organizationID string
	snapshot       Snapshot
}

const ledgerColumns = `organization_id::text, token_limit, reserved_tokens, used_tokens, paused, call_limit,
	agent_token_limit, agent_reserved_tokens, agent_used_tokens, agent_call_limit, agent_calls,
	security_token_limit, security_reserved_tokens, security_used_tokens, security_call_limit, security_calls,
	max_concurrent_calls, calls_in_flight, request_timeout_ms`

func scanLedger(row pgx.Row) (ledgerRow, error) {
	var ledger ledgerRow
	var requestTimeoutMilliseconds int64
	snapshot := &ledger.snapshot
	err := row.Scan(&ledger.organizationID, &snapshot.Limit, &snapshot.Reserved, &snapshot.Used, &snapshot.Paused, &snapshot.CallLimit,
		&snapshot.Agent.TokenLimit, &snapshot.Agent.ReservedTokens, &snapshot.Agent.UsedTokens, &snapshot.Agent.CallLimit, &snapshot.Agent.Calls,
		&snapshot.Security.TokenLimit, &snapshot.Security.ReservedTokens, &snapshot.Security.UsedTokens, &snapshot.Security.CallLimit, &snapshot.Security.Calls,
		&snapshot.MaxConcurrentCalls, &snapshot.CallsInFlight, &requestTimeoutMilliseconds)
	if err != nil {
		return ledgerRow{}, storageError(err)
	}
	snapshot.RequestTimeout = time.Duration(requestTimeoutMilliseconds) * time.Millisecond
	return ledger, nil
}

// Snapshot reads the run's ledger, including how many calls per purpose are unresolved.
func (store *PostgresStore) Snapshot(ctx context.Context, runID string) (Snapshot, error) {
	if ctx == nil || runID == "" {
		return Snapshot{}, ErrInvalid
	}
	if store.pool == nil {
		return Snapshot{}, ErrUnavailable
	}
	ledger, err := scanLedger(store.pool.QueryRow(ctx, `SELECT `+ledgerColumns+` FROM runtime.model_token_budgets WHERE run_id = $1`, runID))
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := ledger.snapshot
	err = store.pool.QueryRow(ctx, `SELECT
			count(*) FILTER (WHERE purpose = 'agent' AND status = 'usage_unknown'),
			count(*) FILTER (WHERE purpose = 'security' AND status = 'usage_unknown')
		FROM runtime.model_token_reservations WHERE run_id = $1`, runID).
		Scan(&snapshot.Agent.UnresolvedCalls, &snapshot.Security.UnresolvedCalls)
	if err != nil {
		return Snapshot{}, ErrUnavailable
	}
	return snapshot, nil
}

// CountAgentCalls returns the run's reserved agent calls: the loop's step count. A call counts once
// its reservation is granted and is never refunded, so a refused reservation is no step.
func (store *PostgresStore) CountAgentCalls(ctx context.Context, organizationID, runID string) (int, error) {
	if ctx == nil || organizationID == "" || runID == "" {
		return 0, ErrInvalid
	}
	if store.pool == nil {
		return 0, ErrUnavailable
	}
	var calls int64
	err := store.pool.QueryRow(ctx, `SELECT agent_calls FROM runtime.model_token_budgets
		WHERE run_id = $1 AND organization_id = $2`, runID, organizationID).Scan(&calls)
	if err != nil {
		return 0, storageError(err)
	}
	return int(calls), nil
}

// lock opens a transaction and locks the run's ledger row.
func (store *PostgresStore) lock(ctx context.Context, runID string) (pgx.Tx, ledgerRow, error) {
	if store.pool == nil {
		return nil, ledgerRow{}, ErrUnavailable
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, ledgerRow{}, ErrUnavailable
	}
	ledger, err := scanLedger(transaction.QueryRow(ctx, `SELECT `+ledgerColumns+` FROM runtime.model_token_budgets WHERE run_id = $1 FOR UPDATE`, runID))
	if err != nil {
		_ = transaction.Rollback(ctx)
		return nil, ledgerRow{}, err
	}
	return transaction, ledger, nil
}

// purposeOf returns the purpose's share of a snapshot.
func purposeOf(snapshot *Snapshot, purpose string) *PurposeUsage {
	if purpose == PurposeAgent {
		return &snapshot.Agent
	}
	return &snapshot.Security
}

// Reserve grants one call of the purpose before dispatch, or refuses with nothing written: the call
// must not be reserved already, the ledger not paused, the shared and purpose call counts and token
// allowances must cover it, and a concurrency slot must be free. The call's dispatch record must
// already exist (runtime.model_calls, same id, organization and purpose).
func (store *PostgresStore) Reserve(ctx context.Context, runID, callID, purpose string, tokens int64) (Reservation, error) {
	if ctx == nil || runID == "" || callID == "" || (purpose != PurposeAgent && purpose != PurposeSecurity) || tokens <= 0 {
		return Reservation{}, ErrInvalid
	}
	transaction, ledger, err := store.lock(ctx, runID)
	if err != nil {
		return Reservation{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var exists bool
	err = transaction.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM runtime.model_token_reservations WHERE run_id = $1 AND call_id = $2)`,
		runID, callID).Scan(&exists)
	if err != nil {
		return Reservation{}, ErrUnavailable
	}
	if exists {
		return Reservation{}, ErrDuplicate
	}
	balance := ledger.snapshot
	share := purposeOf(&balance, purpose)
	switch {
	case balance.Paused:
		return Reservation{}, ErrPaused
	case balance.Agent.Calls+balance.Security.Calls >= balance.CallLimit:
		return Reservation{}, fmt.Errorf("%w: model call limit", ErrExhausted)
	case share.Calls >= share.CallLimit:
		return Reservation{}, fmt.Errorf("%w: %s call limit", ErrExhausted, purpose)
	case balance.Used > balance.Limit || balance.Reserved > balance.Limit-balance.Used || tokens > balance.Limit-balance.Used-balance.Reserved:
		return Reservation{}, fmt.Errorf("%w: shared tokens", ErrExhausted)
	case share.TokenLimit != nil && (share.UsedTokens > *share.TokenLimit || share.ReservedTokens > *share.TokenLimit-share.UsedTokens ||
		tokens > *share.TokenLimit-share.UsedTokens-share.ReservedTokens):
		return Reservation{}, fmt.Errorf("%w: %s tokens", ErrExhausted, purpose)
	case balance.CallsInFlight >= balance.MaxConcurrentCalls:
		return Reservation{}, ErrConcurrencyLimit
	}
	_, err = transaction.Exec(ctx, `
		INSERT INTO runtime.model_token_reservations(run_id, organization_id, call_id, purpose, token_reservation, status,
			slot_held, request_deadline_at)
		VALUES ($1, $2, $3, $4, $5, 'reserved', true, now() + ($6 * interval '1 millisecond'))`,
		runID, ledger.organizationID, callID, purpose, tokens, balance.RequestTimeout.Milliseconds())
	if err != nil {
		// Includes a call id without a dispatch record of this purpose (foreign key).
		return Reservation{}, ErrUnavailable
	}
	_, err = transaction.Exec(ctx, `
		UPDATE runtime.model_token_budgets SET
			reserved_tokens = reserved_tokens + $2,
			agent_reserved_tokens = agent_reserved_tokens + CASE WHEN $3 = 'agent' THEN $2 ELSE 0 END,
			security_reserved_tokens = security_reserved_tokens + CASE WHEN $3 = 'security' THEN $2 ELSE 0 END,
			agent_calls = agent_calls + CASE WHEN $3 = 'agent' THEN 1 ELSE 0 END,
			security_calls = security_calls + CASE WHEN $3 = 'security' THEN 1 ELSE 0 END,
			calls_in_flight = calls_in_flight + 1
		WHERE run_id = $1`, runID, tokens, purpose)
	if err != nil {
		return Reservation{}, ErrUnavailable
	}
	if transaction.Commit(ctx) != nil {
		return Reservation{}, ErrUnavailable
	}
	return Reservation{Tokens: tokens, RequestTimeout: balance.RequestTimeout}, nil
}

// MarkUnknown keeps a dispatched call's whole reservation and its slot: a timeout or missing usage
// does not prove the provider stopped or that nothing was used.
func (store *PostgresStore) MarkUnknown(ctx context.Context, runID, callID string) error {
	if ctx == nil || runID == "" || callID == "" {
		return ErrInvalid
	}
	transaction, _, err := store.lock(ctx, runID)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	tag, err := transaction.Exec(ctx, `UPDATE runtime.model_token_reservations
		SET status = CASE WHEN status = 'settled' THEN status ELSE 'usage_unknown' END
		WHERE run_id = $1 AND call_id = $2`, runID, callID)
	if err != nil {
		return ErrUnavailable
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	if transaction.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}

// Settle records a call's measured usage once, releases its reservation and its slot, and refunds
// the unused tokens. A measured count above the reservation is recorded in full and pauses the
// ledger. Settling again with the same counters reports AlreadySettled; other counters conflict.
func (store *PostgresStore) Settle(ctx context.Context, runID, callID string, inputTokens, outputTokens int64) (Settlement, error) {
	if ctx == nil || runID == "" || callID == "" || inputTokens < 0 || outputTokens < 0 || inputTokens > math.MaxInt64-outputTokens {
		return Settlement{}, ErrInvalid
	}
	actualTokens := inputTokens + outputTokens
	transaction, ledger, err := store.lock(ctx, runID)
	if err != nil {
		return Settlement{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var reservedTokens int64
	var status, purpose string
	var slotHeld bool
	var previousInput, previousOutput *int64
	err = transaction.QueryRow(ctx, `SELECT token_reservation, status, purpose, slot_held, input_tokens, output_tokens
		FROM runtime.model_token_reservations WHERE run_id = $1 AND call_id = $2 FOR UPDATE`, runID, callID).
		Scan(&reservedTokens, &status, &purpose, &slotHeld, &previousInput, &previousOutput)
	if err != nil {
		return Settlement{}, storageError(err)
	}
	balance := ledger.snapshot
	result := Settlement{ActualTokens: actualTokens, Paused: balance.Paused}
	if actualTokens < reservedTokens {
		result.RefundedTokens = reservedTokens - actualTokens
	}
	if status == "settled" {
		if previousInput == nil || previousOutput == nil || *previousInput != inputTokens || *previousOutput != outputTokens {
			return Settlement{}, ErrConflict
		}
		result.AlreadySettled = true
		return result, nil
	}
	share := purposeOf(&balance, purpose)
	if reservedTokens > balance.Reserved || reservedTokens > share.ReservedTokens {
		return Settlement{}, ErrInvalid
	}
	if actualTokens > math.MaxInt64-balance.Used || actualTokens > math.MaxInt64-share.UsedTokens {
		// The measured count cannot fit the aggregate. Fence dispatch durably while retaining the
		// reservation; never clip usage or release an unknown balance.
		if _, err = transaction.Exec(ctx, `UPDATE runtime.model_token_budgets SET paused = true WHERE run_id = $1`, runID); err != nil {
			return Settlement{}, ErrUnavailable
		}
		if _, err = transaction.Exec(ctx, `UPDATE runtime.model_token_reservations SET status = 'usage_unknown'
			WHERE run_id = $1 AND call_id = $2`, runID, callID); err != nil {
			return Settlement{}, ErrUnavailable
		}
		if transaction.Commit(ctx) != nil {
			return Settlement{}, ErrUnavailable
		}
		return Settlement{ActualTokens: actualTokens, Paused: true}, ErrInvalid
	}
	result.Paused = balance.Paused || actualTokens > reservedTokens
	slotRelease := 0
	if slotHeld {
		slotRelease = 1
	}
	_, err = transaction.Exec(ctx, `
		UPDATE runtime.model_token_budgets SET
			reserved_tokens = reserved_tokens - $2,
			used_tokens = used_tokens + $3,
			agent_reserved_tokens = agent_reserved_tokens - CASE WHEN $5 = 'agent' THEN $2 ELSE 0 END,
			agent_used_tokens = agent_used_tokens + CASE WHEN $5 = 'agent' THEN $3 ELSE 0 END,
			security_reserved_tokens = security_reserved_tokens - CASE WHEN $5 = 'security' THEN $2 ELSE 0 END,
			security_used_tokens = security_used_tokens + CASE WHEN $5 = 'security' THEN $3 ELSE 0 END,
			calls_in_flight = calls_in_flight - $6,
			paused = $4
		WHERE run_id = $1`, runID, reservedTokens, actualTokens, result.Paused, purpose, slotRelease)
	if err != nil {
		return Settlement{}, ErrUnavailable
	}
	_, err = transaction.Exec(ctx, `UPDATE runtime.model_token_reservations
		SET status = 'settled', input_tokens = $3, output_tokens = $4, actual_tokens = $5, slot_held = false
		WHERE run_id = $1 AND call_id = $2`, runID, callID, inputTokens, outputTokens, actualTokens)
	if err != nil {
		return Settlement{}, ErrUnavailable
	}
	if transaction.Commit(ctx) != nil {
		return Settlement{}, ErrUnavailable
	}
	return result, nil
}
