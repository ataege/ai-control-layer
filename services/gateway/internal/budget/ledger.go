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
// The per-purpose token sub-limits are validated here; they are stored once GO-39's migration
// adds their columns. The X-08 passport fixture leaves them null.
func OpenRunLedger(ctx context.Context, transaction pgx.Tx, organizationID, runID string, limits contracts.PassportLimits) error {
	if ctx == nil || transaction == nil || !ledgerUUIDPattern.MatchString(organizationID) || !ledgerUUIDPattern.MatchString(runID) {
		return ErrInvalid
	}
	if limits.TokensTotal <= 0 || limits.TokensTotal > maximumSafeInteger ||
		!validSubLimit(limits.TokensAgent, limits.TokensTotal) || !validSubLimit(limits.TokensSecurity, limits.TokensTotal) {
		return ErrInvalid
	}
	if _, err := transaction.Exec(ctx,
		"INSERT INTO runtime.model_token_budgets(run_id, token_limit) VALUES ($1, $2)", runID, limits.TokensTotal); err != nil {
		return ErrUnavailable
	}
	return nil
}

// validSubLimit accepts an absent sub-limit or a positive one inside the shared total.
func validSubLimit(subLimit *int64, total int64) bool {
	return subLimit == nil || (*subLimit > 0 && *subLimit <= total)
}
