package reads

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// RunUsage is the usage half of the run view (X-29), the Go side's draft until the shared
// contract lands it. It counts recorded dispatches and ledger rows only: no estimate, and no cost,
// because the local model has no billed price to report. Usage that is unknown (a timeout, a
// transport error) stays visible as its own count and keeps its reservation; it is never zero.
type RunUsage struct {
	RunID string `json:"runId"`
	// ModelCalls has one entry per metered purpose, always "agent" then "security".
	ModelCalls []PurposeUsage `json:"modelCalls"`
	// Ledger is the run's model ledger (calls, tokens, request time, slots); null when the run has
	// no ledger yet.
	Ledger       *ModelLedger     `json:"ledger"`
	ToolAttempts ToolAttemptUsage `json:"toolAttempts"`
}

// PurposeUsage is the model usage of one metered purpose.
type PurposeUsage struct {
	// Purpose is "agent" or "security".
	Purpose string `json:"purpose"`
	// Dispatched counts the dispatch records committed before each call; by outcome below.
	Dispatched   int64 `json:"dispatched"`
	Completed    int64 `json:"completed"`
	Failed       int64 `json:"failed"`
	UsageUnknown int64 `json:"usageUnknown"`
	// InFlight: dispatched, no outcome recorded yet.
	InFlight int64 `json:"inFlight"`
	// SettledTokens are the provider-reported tokens of settled calls.
	SettledTokens int64 `json:"settledTokens"`
	// HeldTokens are reserved for calls that are in flight or whose usage is unknown.
	HeldTokens int64 `json:"heldTokens"`
	// UsageUnknownReservations count reservations kept because usage could not be settled.
	UsageUnknownReservations int64 `json:"usageUnknownReservations"`
}

// ModelLedger is the run's model allowance as the ledger holds it (GO-39): the single authority
// for model calls, tokens, request time and concurrent calls.
type ModelLedger struct {
	Paused bool `json:"paused"`
	// Tokens is the shared token total of both purposes.
	Tokens         LedgerTokens `json:"tokens"`
	AgentTokens    LedgerTokens `json:"agentTokens"`
	SecurityTokens LedgerTokens `json:"securityTokens"`
	Calls          LedgerCalls  `json:"calls"`
	// RequestTimeoutMilliseconds bounds each model request.
	RequestTimeoutMilliseconds int64 `json:"requestTimeoutMilliseconds"`
	MaxConcurrentCalls         int64 `json:"maxConcurrentCalls"`
	// CallsInFlight are held slots: calls in flight or with unknown usage.
	CallsInFlight int64 `json:"callsInFlight"`
}

// LedgerTokens is one token allowance. Limit is null for a purpose without its own sub-limit
// (only the shared total applies).
type LedgerTokens struct {
	Limit    *int64 `json:"limit"`
	Reserved int64  `json:"reserved"`
	Used     int64  `json:"used"`
}

// LedgerCalls are the call limits and the calls counted at reservation (never refunded).
type LedgerCalls struct {
	Limit         int64 `json:"limit"`
	AgentLimit    int64 `json:"agentLimit"`
	SecurityLimit int64 `json:"securityLimit"`
	Agent         int64 `json:"agent"`
	Security      int64 `json:"security"`
}

// ToolAttemptUsage counts the run's tool execution attempts by outcome. Open attempts were
// dispatched and have no recorded outcome: their effect is not known yet.
type ToolAttemptUsage struct {
	Total     int64 `json:"total"`
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
	Aborted   int64 `json:"aborted"`
	Open      int64 `json:"open"`
}

// errRunNotFound means the run is not one of the organization's runs.
var errRunNotFound = errors.New("reads: run not found")

// meteredPurposes are the model purposes in display order.
var meteredPurposes = []string{"agent", "security"}

// RunUsageHandler serves the usage of one run of the operator's organization.
func RunUsageHandler(database Beginner) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		organizationID, runID, ok := runRequest(responseWriter, request)
		if !ok {
			return
		}
		var usage RunUsage
		err := readTransaction(request.Context(), database, func(tx pgx.Tx) error {
			var readErr error
			usage, readErr = readRunUsage(request.Context(), tx, organizationID, runID)
			return readErr
		})
		if err != nil {
			writeReadError(responseWriter, request, err)
			return
		}
		writeJSON(responseWriter, usage)
	})
}

// readRunUsage checks that the run belongs to the organization before it reads its usage; every
// query is also scoped by the organization.
func readRunUsage(ctx context.Context, tx pgx.Tx, organizationID, runID string) (RunUsage, error) {
	var found bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM runtime.runs WHERE id = $1 AND organization_id = $2)`,
		runID, organizationID).Scan(&found)
	if err != nil {
		return RunUsage{}, err
	}
	if !found {
		return RunUsage{}, errRunNotFound
	}
	usage := RunUsage{RunID: runID}
	if usage.ModelCalls, err = readPurposeUsage(ctx, tx, organizationID, &runID); err != nil {
		return RunUsage{}, err
	}
	var ledger ModelLedger
	var tokenLimit int64
	err = tx.QueryRow(ctx, `SELECT paused, token_limit, reserved_tokens, used_tokens,
			agent_token_limit, agent_reserved_tokens, agent_used_tokens,
			security_token_limit, security_reserved_tokens, security_used_tokens,
			call_limit, agent_call_limit, security_call_limit, agent_calls, security_calls,
			request_timeout_ms, max_concurrent_calls, calls_in_flight
		FROM runtime.model_token_budgets WHERE run_id = $1 AND organization_id = $2`, runID, organizationID).
		Scan(&ledger.Paused, &tokenLimit, &ledger.Tokens.Reserved, &ledger.Tokens.Used,
			&ledger.AgentTokens.Limit, &ledger.AgentTokens.Reserved, &ledger.AgentTokens.Used,
			&ledger.SecurityTokens.Limit, &ledger.SecurityTokens.Reserved, &ledger.SecurityTokens.Used,
			&ledger.Calls.Limit, &ledger.Calls.AgentLimit, &ledger.Calls.SecurityLimit, &ledger.Calls.Agent, &ledger.Calls.Security,
			&ledger.RequestTimeoutMilliseconds, &ledger.MaxConcurrentCalls, &ledger.CallsInFlight)
	switch {
	case err == nil:
		ledger.Tokens.Limit = &tokenLimit
		usage.Ledger = &ledger
	case !errors.Is(err, pgx.ErrNoRows):
		return RunUsage{}, err
	}
	var unrecognized int64
	err = tx.QueryRow(ctx, `SELECT count(*),
			count(*) FILTER (WHERE attempt.outcome = 'succeeded'),
			count(*) FILTER (WHERE attempt.outcome = 'failed'),
			count(*) FILTER (WHERE attempt.outcome = 'aborted'),
			count(*) FILTER (WHERE attempt.completed_at IS NULL),
			count(*) FILTER (WHERE attempt.completed_at IS NOT NULL
			  AND attempt.outcome IS DISTINCT FROM 'succeeded' AND attempt.outcome IS DISTINCT FROM 'failed'
			  AND attempt.outcome IS DISTINCT FROM 'aborted')
		FROM runtime.execution_attempts AS attempt
		JOIN runtime.actions AS action
		  ON action.id = attempt.action_id AND action.organization_id = attempt.organization_id
		WHERE attempt.organization_id = $1 AND action.run_id = $2`, organizationID, runID).
		Scan(&usage.ToolAttempts.Total, &usage.ToolAttempts.Succeeded, &usage.ToolAttempts.Failed,
			&usage.ToolAttempts.Aborted, &usage.ToolAttempts.Open, &unrecognized)
	if err != nil {
		return RunUsage{}, err
	}
	if unrecognized > 0 {
		// An outcome this view does not know is never folded into another count.
		return RunUsage{}, errMalformedRecord
	}
	return usage, nil
}

// readPurposeUsage aggregates model usage per purpose for the organization, or for one of its
// runs when runID is set. The ledger rows are also joined to runtime.runs, so only the
// organization's own runs count.
func readPurposeUsage(ctx context.Context, tx pgx.Tx, organizationID string, runID *string) ([]PurposeUsage, error) {
	byPurpose := map[string]*PurposeUsage{}
	usage := make([]PurposeUsage, len(meteredPurposes))
	for index, purpose := range meteredPurposes {
		usage[index] = PurposeUsage{Purpose: purpose}
		byPurpose[purpose] = &usage[index]
	}
	rows, err := tx.Query(ctx, `SELECT purpose, count(*),
			count(*) FILTER (WHERE outcome = 'completed'),
			count(*) FILTER (WHERE outcome = 'failed'),
			count(*) FILTER (WHERE outcome = 'usage_unknown'),
			count(*) FILTER (WHERE outcome IS NULL),
			count(*) FILTER (WHERE outcome NOT IN ('completed', 'failed', 'usage_unknown'))
		FROM runtime.model_calls
		WHERE organization_id = $1 AND ($2::text IS NULL OR run_id = $2::text::uuid)
		GROUP BY purpose`, organizationID, runID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var purpose string
		var counts PurposeUsage
		var unrecognized int64
		if err := rows.Scan(&purpose, &counts.Dispatched, &counts.Completed, &counts.Failed,
			&counts.UsageUnknown, &counts.InFlight, &unrecognized); err != nil {
			rows.Close()
			return nil, err
		}
		target, known := byPurpose[purpose]
		if !known || unrecognized > 0 {
			rows.Close()
			return nil, errMalformedRecord
		}
		target.Dispatched, target.Completed, target.Failed = counts.Dispatched, counts.Completed, counts.Failed
		target.UsageUnknown, target.InFlight = counts.UsageUnknown, counts.InFlight
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = tx.Query(ctx, `SELECT reservation.purpose,
			coalesce(sum(reservation.actual_tokens) FILTER (WHERE reservation.status = 'settled'), 0)::bigint,
			coalesce(sum(reservation.token_reservation) FILTER (WHERE reservation.status IN ('reserved', 'usage_unknown')), 0)::bigint,
			count(*) FILTER (WHERE reservation.status = 'usage_unknown')
		FROM runtime.model_token_reservations AS reservation
		JOIN runtime.runs AS run ON run.id = reservation.run_id AND run.organization_id = reservation.organization_id
		WHERE run.organization_id = $1 AND ($2::text IS NULL OR run.id = $2::text::uuid)
		GROUP BY reservation.purpose`, organizationID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var purpose string
		var settled, held, unknown int64
		if err := rows.Scan(&purpose, &settled, &held, &unknown); err != nil {
			return nil, err
		}
		target, known := byPurpose[purpose]
		if !known {
			return nil, errMalformedRecord
		}
		target.SettledTokens, target.HeldTokens, target.UsageUnknownReservations = settled, held, unknown
	}
	return usage, rows.Err()
}
