package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/catalog/catalogtest"
	"starter/services/gateway/internal/testdb"
)

type pointerRow struct {
	requested, validated, active, feed *int64
	lastError                          []byte
}

func readPointer(t *testing.T, world *catalogWorld) pointerRow {
	t.Helper()
	var row pointerRow
	err := world.outer.QueryRow(context.Background(), `SELECT requested_revision_id, validated_revision_id,
		active_revision_id, active_feed_revision_id, last_error FROM app.control_catalog_pointer WHERE id = 1`).
		Scan(&row.requested, &row.validated, &row.active, &row.feed, &row.lastError)
	if err != nil {
		t.Fatalf("read pointer: %v", err)
	}
	return row
}

func equalID(value *int64, want int64) bool { return value != nil && *value == want }

func TestPostgresValidRequestedRevisionBecomesActive(t *testing.T) {
	world := newCatalogWorld(t)
	activeRevision := world.activate(t, policyContent, &world.feedID)
	requested := catalogtest.Request(t, world.outer, strings.Replace(policyContent, `"threshold": 0.75`, `"threshold": 0.9`, 1))
	if pointer := readPointer(t, world); !equalID(pointer.active, activeRevision) {
		t.Fatal("the import must not activate the revision itself")
	}

	outcome, err := ActivateRequested(context.Background(), world.outer)
	if err != nil || outcome != ActivationActivated {
		t.Fatalf("outcome %s err %v", outcome, err)
	}
	pointer := readPointer(t, world)
	if !equalID(pointer.validated, requested) || !equalID(pointer.active, requested) || !equalID(pointer.feed, world.feedID) || pointer.lastError != nil {
		t.Errorf("pointer after activation: %+v", pointer)
	}
	snapshot, err := NewLoader().Active(context.Background(), world.outer)
	if err != nil || snapshot.RevisionID != requested || snapshot.Security.SemanticInjection.Threshold != 0.9 {
		t.Errorf("snapshot %+v err %v", snapshot, err)
	}
	if outcome, _ := ActivateRequested(context.Background(), world.outer); outcome != ActivationIdle {
		t.Errorf("an active request was handled again: %s", outcome)
	}
}

func TestPostgresInvalidRequestedRevisionKeepsTheLastGoodOne(t *testing.T) {
	cases := map[string]struct {
		content  string
		wantCode string
	}{
		"invalid limits":        {strings.Replace(policyContent, `"calls_agent": 12`, `"calls_agent": 30`, 1), rejectionInvalid},
		"unknown disabled rule": {strings.Replace(policyContent, `"disabled_rules": []`, `"disabled_rules": ["no_such_rule_v1"]`, 1), rejectionInvalid},
		"feed not imported":     {strings.Replace(policyContent, `"revision": "feed_v1"`, `"revision": "feed_v9"`, 1), rejectionFeedMissing},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			world := newCatalogWorld(t)
			activeRevision := world.activate(t, policyContent, &world.feedID)
			requested := catalogtest.Request(t, world.outer, testCase.content)

			outcome, err := ActivateRequested(context.Background(), world.outer)
			if err != nil || outcome != ActivationRejected {
				t.Fatalf("outcome %s err %v", outcome, err)
			}
			pointer := readPointer(t, world)
			if !equalID(pointer.active, activeRevision) || !equalID(pointer.feed, world.feedID) || pointer.validated != nil && *pointer.validated == requested {
				t.Errorf("the rejected revision changed the active state: %+v", pointer)
			}
			var record rejection
			if json.Unmarshal(pointer.lastError, &record) != nil || record.Code != testCase.wantCode ||
				record.RevisionID != requested || record.Reason != "policy_reload_rejected" || record.Message == "" {
				t.Errorf("last_error = %s", pointer.lastError)
			}
			if strings.Contains(string(pointer.lastError), "calls_agent") || strings.Contains(string(pointer.lastError), "no_such_rule") {
				t.Errorf("last_error echoes file content: %s", pointer.lastError)
			}
			snapshot, err := NewLoader().Active(context.Background(), world.outer)
			if err != nil || snapshot.RevisionID != activeRevision {
				t.Errorf("the last good revision no longer decides: %+v %v", snapshot, err)
			}
			// A failed request is not retried; a new import sets a new requested revision.
			if outcome, _ := ActivateRequested(context.Background(), world.outer); outcome != ActivationIdle {
				t.Errorf("a rejected request was retried: %s", outcome)
			}
		})
	}
}

func TestPostgresFirstRevisionActivatesAndDisabledSignaturesNeedNoFeed(t *testing.T) {
	world := newCatalogWorld(t)
	if _, err := world.outer.Exec(context.Background(), `UPDATE app.control_catalog_pointer
		SET active_revision_id = NULL, validated_revision_id = NULL, active_feed_revision_id = NULL WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	requested := catalogtest.Request(t, world.outer, strings.Replace(strings.Replace(policyContent,
		`"signature_match": {"enabled": true`, `"signature_match": {"enabled": false`, 1), `"revision": "feed_v1"`, `"revision": "feed_none"`, 1))
	if outcome, err := ActivateRequested(context.Background(), world.outer); err != nil || outcome != ActivationActivated {
		t.Fatalf("outcome %s err %v", outcome, err)
	}
	if pointer := readPointer(t, world); !equalID(pointer.active, requested) || pointer.feed != nil {
		t.Errorf("pointer %+v", pointer)
	}
}

func TestPostgresActivationIsSkippedWhileAnotherInstanceHoldsTheLock(t *testing.T) {
	world := newCatalogWorld(t)
	world.activate(t, policyContent, &world.feedID)
	catalogtest.Request(t, world.outer, policyContent)
	other, err := testdb.Open(t).Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Rollback(context.Background()) }()
	if _, err := other.Exec(context.Background(), `SELECT pg_advisory_xact_lock($1)`, activationLockKey); err != nil {
		t.Fatal(err)
	}
	if outcome, err := ActivateRequested(context.Background(), world.outer); err != nil || outcome != ActivationBusy {
		t.Errorf("outcome %s err %v, want busy", outcome, err)
	}
}

// The gateway's own database role can acknowledge a revision but not request one.
func TestPostgresGatewayRoleMayOnlyAcknowledge(t *testing.T) {
	world := newCatalogWorld(t)
	world.activate(t, policyContent, &world.feedID)
	requested := catalogtest.Request(t, world.outer, strings.Replace(policyContent, `"threshold": 0.75`, `"threshold": 0.8`, 1))
	if _, err := world.outer.Exec(context.Background(), `SET LOCAL ROLE task_passport_gateway`); err != nil {
		t.Fatalf("set role (run the migrations): %v", err)
	}
	if outcome, err := ActivateRequested(context.Background(), world.outer); err != nil || outcome != ActivationActivated {
		t.Fatalf("activation as the gateway role: %s %v", outcome, err)
	}
	if pointer := readPointer(t, world); !equalID(pointer.active, requested) {
		t.Errorf("pointer %+v", pointer)
	}
	for name, statement := range map[string]string{
		"request a revision": `UPDATE app.control_catalog_pointer SET requested_revision_id = NULL WHERE id = 1`,
		"import a revision": `INSERT INTO app.control_catalog_revisions (schema_version, source_file_name, source_text,
			file_digest, content, import_source) VALUES (1, 'x', 'x', repeat('e', 64), '{}', 'command')`,
	} {
		savepoint, err := world.outer.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_, err = savepoint.Exec(context.Background(), statement)
		_ = savepoint.Rollback(context.Background())
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s as the gateway role: %v", name, err)
		}
	}
}

// failingDatabase stands in for a database whose transactions cannot start.
type failingDatabase struct{}

func (failingDatabase) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("connection refused")
}

// A lasting activation failure is named once at the default level, not on every tick and not
// only at Debug, so an import that can never activate is visible.
func TestWatchRequestedWarnsOnceWhileChecksFail(t *testing.T) {
	var logged strings.Builder
	logger := slog.New(slog.NewTextHandler(&logged, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	WatchRequested(ctx, failingDatabase{}, 10*time.Millisecond, logger)
	if count := strings.Count(logged.String(), "level=WARN msg=\"catalog activation checks are failing"); count != 1 {
		t.Errorf("warnings = %d, want 1; log:\n%s", count, logged.String())
	}
	if strings.Contains(logged.String(), "connection refused") {
		t.Error("the driver error reached the log")
	}
}
