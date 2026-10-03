package budget

import (
	"context"
	"regexp"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
)

// maximumSafeInteger is the largest integer TypeScript and JSON consumers read exactly.
const maximumSafeInteger int64 = 9007199254740991

var ledgerUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// OpenRunLedger opens the run's token ledger inside the caller's transaction, so admission
// commits it together with the passport, run and job (alignment decision 6: the token limit is
// copied from the passport at admission and never raised afterwards). A run without a ledger row
// cannot reserve, so no model request is dispatched for it.
//
// All the passport's model limits are copied: the shared token total, the per-purpose token
// sub-limits (nullable, as in X-08), the call limits, the request timeout and the concurrency cap.
func OpenRunLedger(ctx context.Context, transaction pgx.Tx, organizationID, runID string, limits contracts.PassportLimits) error {
	if ctx == nil || transaction == nil || !ledgerUUIDPattern.MatchString(organizationID) || !ledgerUUIDPattern.MatchString(runID) {
		return ErrInvalid
	}
	if limits.TokensTotal <= 0 || limits.TokensTotal > maximumSafeInteger ||
		!validSubLimit(limits.TokensAgent, limits.TokensTotal) || !validSubLimit(limits.TokensSecurity, limits.TokensTotal) ||
		limits.CallsTotal <= 0 || limits.CallsTotal > maximumSafeInteger ||
		limits.CallsAgent < 0 || limits.CallsAgent > limits.CallsTotal ||
		limits.CallsSecurity < 0 || limits.CallsSecurity > limits.CallsTotal ||
		limits.RequestTimeoutSeconds <= 0 || limits.RequestTimeoutSeconds > 3600 ||
		limits.LocalMaxConcurrency <= 0 || limits.LocalMaxConcurrency > 1024 {
		return ErrInvalid
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO runtime.model_token_budgets(run_id, organization_id, token_limit, agent_token_limit, security_token_limit,
			call_limit, agent_call_limit, security_call_limit, request_timeout_ms, max_concurrent_calls)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		runID, organizationID, limits.TokensTotal, limits.TokensAgent, limits.TokensSecurity,
		limits.CallsTotal, limits.CallsAgent, limits.CallsSecurity, limits.RequestTimeoutSeconds*1000, limits.LocalMaxConcurrency); err != nil {
		return ErrUnavailable
	}
	return nil
}

// validSubLimit accepts an absent sub-limit or a positive one inside the shared total.
func validSubLimit(subLimit *int64, total int64) bool {
	return subLimit == nil || (*subLimit > 0 && *subLimit <= total)
}
