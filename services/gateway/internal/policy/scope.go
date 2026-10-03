package policy

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/repository"
)

// ErrScopeUnavailable means the passport or the active catalog could not be read; the gate then
// denies.
var ErrScopeUnavailable = errors.New("passport scope unavailable")

// PassportScopeReader loads the gate's scope from the run's stored passport, decoded strictly
// against X-08 by the runtime repository, and the active catalog revision.
type PassportScopeReader struct {
	pool       *pgxpool.Pool
	repository *repository.Repository
}

// NewPassportScopeReader returns a reader on the given pool. It creates nothing.
func NewPassportScopeReader(pool *pgxpool.Pool) *PassportScopeReader {
	return &PassportScopeReader{pool: pool, repository: repository.New(pool)}
}

// LoadScope returns the passport scope of the verified run. A passport that no longer fits its
// contract is never used as authority (the repository refuses it).
func (reader *PassportScopeReader) LoadScope(ctx context.Context, run RunIdentity) (PassportScope, error) {
	if reader.pool == nil {
		return PassportScope{}, ErrScopeUnavailable
	}
	passport, err := reader.repository.Passport(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return PassportScope{}, ErrScopeUnavailable
	}
	templates := make([]string, 0, len(passport.Scope.ReportTemplates))
	for _, template := range passport.Scope.ReportTemplates {
		templates = append(templates, string(template))
	}
	return PassportScope{
		OrganizationID:             passport.OrganizationID,
		RunID:                      passport.RunID,
		PassportID:                 passport.PassportID,
		AllowedTools:               passport.Scope.Tools,
		AllowedInvoiceIDs:          passport.Scope.InvoiceIDs,
		AllowedTemplates:           templates,
		RecipientReferences:        passport.Scope.RecipientReferences,
		ApprovalRequiredTools:      passport.Scope.ApprovalRequiredTools,
		ToolAttemptLimit:           int(passport.Limits.ToolAttempts),
		AdmissionCatalogRevisionID: passport.AdmissionCatalogRevisionID,
		ExpiresAt:                  passport.ExpiresAt,
	}, nil
}

// ActiveCatalogRevision returns the active control-catalog revision, validated as enforceable;
// a missing or invalid catalog is an error, never a default.
func (reader *PassportScopeReader) ActiveCatalogRevision(ctx context.Context) (int64, error) {
	if reader.pool == nil {
		return 0, ErrScopeUnavailable
	}
	catalog, err := config.ReadActiveAccountingCatalog(ctx, reader.pool)
	if err != nil {
		return 0, ErrScopeUnavailable
	}
	return catalog.RevisionID, nil
}
