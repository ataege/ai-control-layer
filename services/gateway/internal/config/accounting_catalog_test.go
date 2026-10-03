package config

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// The reader double exercises unavailable and malformed active snapshots without
// importing or activating a policy. Live catalog import is tested by test:db.
type accountingReaderDouble struct {
	revision int64
	version  int
	raw      []byte
	err      error
}

func (reader accountingReaderDouble) QueryRow(context.Context, string, ...any) pgx.Row { return reader }
func (reader accountingReaderDouble) Scan(destinations ...any) error {
	if reader.err != nil {
		return reader.err
	}
	*destinations[0].(*int64) = reader.revision
	*destinations[1].(*int) = reader.version
	*destinations[2].(*[]byte) = reader.raw
	return nil
}

func TestActiveAccountingCatalogFailsClosed(t *testing.T) {
	valid := []byte(`{"schema_version":1,"allowed_models":["fixture:tag"],"budgets":{"tokens_total":20000,"request_timeout_seconds":20}}`)
	for _, reader := range []CatalogReader{
		nil,
		accountingReaderDouble{err: errors.New("postgresql://private-credential")},
		accountingReaderDouble{revision: 1, version: 2, raw: valid},
		accountingReaderDouble{revision: 0, version: 1, raw: valid},
		accountingReaderDouble{revision: 1, version: 1, raw: []byte(`{"budgets":{"tokens_total":0}}`)},
	} {
		_, err := ReadActiveAccountingCatalog(context.Background(), reader)
		if !errors.Is(err, ErrAccountingCatalog) || strings.Contains(err.Error(), "private-credential") {
			t.Fatalf("catalog did not fail safely: %v", err)
		}
	}
	catalog, err := ReadActiveAccountingCatalog(context.Background(), accountingReaderDouble{revision: 42, version: 1, raw: valid})
	if err != nil || catalog.RevisionID != 42 || catalog.Settings.TokensTotal != 20000 {
		t.Fatalf("active snapshot not preserved: %+v %v", catalog, err)
	}
}
