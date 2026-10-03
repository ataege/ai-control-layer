package config

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrAccountingCatalog = errors.New("active model accounting catalog unavailable")

type CatalogReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type AccountingCatalog struct {
	RevisionID int64
	Settings   ModelAccounting
}

// ReadActiveAccountingCatalog reads one coherent active-revision snapshot. It
// neither imports policy files nor activates candidates; GO-72/73 own the full
// policy/feed enforcement path. This projection is also used by budgetcheck.
func ReadActiveAccountingCatalog(ctx context.Context, reader CatalogReader) (AccountingCatalog, error) {
	var catalog AccountingCatalog
	if ctx == nil || reader == nil {
		return catalog, ErrAccountingCatalog
	}
	var raw []byte
	var version int
	err := reader.QueryRow(ctx, `SELECT revision.id, revision.schema_version, revision.content
		FROM app.control_catalog_pointer AS pointer
		JOIN app.control_catalog_revisions AS revision ON revision.id = pointer.active_revision_id
		WHERE pointer.id = 1`).Scan(&catalog.RevisionID, &version, &raw)
	if err != nil || catalog.RevisionID <= 0 || version != 1 {
		return AccountingCatalog{}, ErrAccountingCatalog
	}
	catalog.Settings, err = LoadAccountingCatalog(raw)
	if err != nil {
		return AccountingCatalog{}, ErrAccountingCatalog
	}
	return catalog, nil
}
