package policy

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/contracts"
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
	catalogs   activeSnapshots
}

// activeSnapshots is the active catalog source; *catalog.Loader in production.
type activeSnapshots interface {
	Active(ctx context.Context, querier catalog.Querier) (catalog.Snapshot, error)
}

// NewPassportScopeReader returns a reader on the given pool. It creates nothing.
func NewPassportScopeReader(pool *pgxpool.Pool) *PassportScopeReader {
	return &PassportScopeReader{pool: pool, repository: repository.New(pool), catalogs: catalog.NewLoader()}
}

// LoadScope returns the passport scope of the verified run, narrowed by the active catalog. A
// passport that no longer fits its contract is never used as authority (the repository refuses
// it), and without an enforceable active catalog there is no scope: the gate denies.
func (reader *PassportScopeReader) LoadScope(ctx context.Context, run RunIdentity) (PassportScope, error) {
	if reader.pool == nil {
		return PassportScope{}, ErrScopeUnavailable
	}
	passport, err := reader.repository.Passport(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return PassportScope{}, ErrScopeUnavailable
	}
	snapshot, err := reader.catalogs.Active(ctx, reader.pool)
	if err != nil {
		return PassportScope{}, ErrScopeUnavailable
	}
	// The catalog only narrows the passport: templates and the tool-attempt ceiling (config/README.md,
	// "Budget changes apply as current restrictions").
	effective := catalog.EffectiveFor(passport, snapshot)
	return PassportScope{
		OrganizationID:             passport.OrganizationID,
		RunID:                      passport.RunID,
		PassportID:                 passport.PassportID,
		AllowedTools:               passport.Scope.Tools,
		AllowedInvoiceIDs:          passport.Scope.InvoiceIDs,
		AllowedTemplates:           templateNames(effective.ReportTemplates),
		RecipientReferences:        passport.Scope.RecipientReferences,
		ApprovalRequiredTools:      passport.Scope.ApprovalRequiredTools,
		ToolAttemptLimit:           int(effective.ToolAttempts),
		AdmissionCatalogRevisionID: passport.AdmissionCatalogRevisionID,
		ExpiresAt:                  passport.ExpiresAt,
	}, nil
}

// ActiveCatalogRevision returns the active control-catalog revision through catalog.Loader, the
// single active-snapshot source, which validates the snapshot as enforceable; a missing or
// invalid catalog is an error, never a default.
// LoadScope and ActiveCatalogRevision read the snapshot separately, so a reload between the two
// can pair revision N's templates with revision N+1 for one decision. An approved action is
// rechecked against the active revision at execution (source_policy_changed), which closes that
// window for every effect that needs review.
func (reader *PassportScopeReader) ActiveCatalogRevision(ctx context.Context) (int64, error) {
	if reader.pool == nil {
		return 0, ErrScopeUnavailable
	}
	snapshot, err := reader.catalogs.Active(ctx, reader.pool)
	if err != nil {
		return 0, ErrScopeUnavailable
	}
	return snapshot.RevisionID, nil
}

// templateNames lists the report templates that the passport allows and the active catalog still
// enables (GO-70/GO-72): a template disabled in reports.enabled_templates after admission is refused
// at the gate with template_not_allowed. It only narrows: a template the catalog enables but the
// passport lacks is never added.
func templateNames(templates []contracts.ReportTemplate) []string {
	names := make([]string, 0, len(templates))
	for _, template := range templates {
		names = append(names, string(template))
	}
	return names
}
