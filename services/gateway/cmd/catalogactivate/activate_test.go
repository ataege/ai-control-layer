package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/catalog/catalogtest"
	"starter/services/gateway/internal/testdb"
)

// world is one rolled-back transaction holding the trusted feed and, per test, a catalog state.
type world struct {
	outer  pgx.Tx
	feedID int64
}

func newWorld(t *testing.T) *world {
	t.Helper()
	pool := testdb.Open(t)
	outer, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = outer.Rollback(context.Background()) })
	return &world{outer: outer, feedID: catalogtest.InsertFeed(t, outer)}
}

func (w *world) run() (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = activate(context.Background(), w.outer, &out, &errOut)
	return code, out.String(), errOut.String()
}

func (w *world) pointer(t *testing.T) (requested, active, feed *int64) {
	t.Helper()
	if err := w.outer.QueryRow(context.Background(), `SELECT requested_revision_id, active_revision_id, active_feed_revision_id
		FROM app.control_catalog_pointer WHERE id = 1`).Scan(&requested, &active, &feed); err != nil {
		t.Fatalf("read pointer: %v", err)
	}
	return requested, active, feed
}

func (w *world) clearPointer(t *testing.T) {
	t.Helper()
	if _, err := w.outer.Exec(context.Background(), `DELETE FROM app.control_catalog_pointer`); err != nil {
		t.Fatalf("clear the pointer: %v", err)
	}
}

// A requested revision that validates is activated with its feed, and nothing but fixed texts and
// ids is printed.
func TestPostgresActivatesTheRequestedRevision(t *testing.T) {
	w := newWorld(t)
	w.clearPointer(t)
	requested := catalogtest.Request(t, w.outer, catalogtest.PolicyContent)

	code, stdout, stderr := w.run()
	if code != 0 || stderr != "" || !strings.Contains(stdout, "activated revision") {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout, stderr)
	}
	_, active, feed := w.pointer(t)
	if active == nil || *active != requested || feed == nil || *feed != w.feedID {
		t.Errorf("active %v feed %v, want %d and %d", active, feed, requested, w.feedID)
	}
	for _, leak := range []string{"threshold", "qwen", "feed_v1"} {
		if strings.Contains(stdout+stderr, leak) {
			t.Errorf("output mentions %q: %s%s", leak, stdout, stderr)
		}
	}
	// A second run finds nothing to do and still succeeds.
	if code, stdout, _ := w.run(); code != 0 || !strings.Contains(stdout, "nothing to activate") {
		t.Errorf("second run: code %d stdout %q", code, stdout)
	}
}

// No pointer row, or a pointer with nothing active, is a failure: the catalog is not enforceable.
func TestPostgresFailsWhenNothingIsActive(t *testing.T) {
	w := newWorld(t)
	w.clearPointer(t)
	if code, _, stderr := w.run(); code != 1 || !strings.Contains(stderr, "no active catalog") {
		t.Errorf("no pointer: code %d stderr %q", code, stderr)
	}
}

// A rejected request exits 1 with its safe code, keeps the last good revision, and is still
// reported on a second run (the gateway does not retry it; a new import is the fix).
func TestPostgresRejectedRequestFailsAndKeepsTheLastGoodRevision(t *testing.T) {
	w := newWorld(t)
	good := catalogtest.Activate(t, w.outer, catalogtest.PolicyContent, &w.feedID)
	catalogtest.Request(t, w.outer, strings.Replace(catalogtest.PolicyContent, `"revision": "feed_v1"`, `"revision": "feed_v9"`, 1))

	for attempt := 1; attempt <= 2; attempt++ {
		code, stdout, stderr := w.run()
		if code != 1 || stdout != "" || !strings.Contains(stderr, "was rejected, code: signature_feed_missing") || !strings.Contains(stderr, "stays active") {
			t.Fatalf("attempt %d: code %d stdout %q stderr %q", attempt, code, stdout, stderr)
		}
		if _, active, _ := w.pointer(t); active == nil || *active != good {
			t.Fatalf("attempt %d: the last good revision changed: %v", attempt, active)
		}
	}
}

func TestPostgresFirstRejectedRequestLeavesNoActiveCatalog(t *testing.T) {
	w := newWorld(t)
	w.clearPointer(t)
	catalogtest.Request(t, w.outer, strings.Replace(catalogtest.PolicyContent, `"calls_agent": 12`, `"calls_agent": 30`, 1))
	code, _, stderr := w.run()
	if code != 1 || !strings.Contains(stderr, "no active catalog, code: catalog_invalid") {
		t.Errorf("code %d stderr %q", code, stderr)
	}
}

// The state an old import's first-revision bootstrap leaves (3c's rehearsal): requested = active,
// validated and feed empty. The activation sees nothing pending, but admission would fail closed, so
// the command must not report success.
func TestPostgresBootstrappedUnvalidatedCatalogFails(t *testing.T) {
	w := newWorld(t)
	active := catalogtest.Activate(t, w.outer, catalogtest.PolicyContent, nil)
	if _, err := w.outer.Exec(context.Background(), `UPDATE app.control_catalog_pointer SET requested_revision_id = $1 WHERE id = 1`, active); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := w.run()
	if code != 1 || stdout != "" || !strings.Contains(stderr, "never validated by the gateway") {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout, stderr)
	}
	// A validated active revision without the feed its policy needs still cannot be loaded.
	if _, err := w.outer.Exec(context.Background(), `UPDATE app.control_catalog_pointer SET validated_revision_id = active_revision_id WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := w.run(); code != 1 || !strings.Contains(stderr, "cannot be loaded as an enforceable catalog") {
		t.Fatalf("validated but feedless: code %d stderr %q", code, stderr)
	}
}

// A policy with signature matching off needs no feed, so a gateway-validated revision without one is
// a legitimate enforceable catalog.
func TestPostgresSignaturesOffPolicyNeedsNoFeed(t *testing.T) {
	w := newWorld(t)
	w.clearPointer(t)
	content := strings.Replace(catalogtest.PolicyContent,
		`"signature_match": {"enabled": true,`, `"signature_match": {"enabled": false,`, 1)
	catalogtest.Request(t, w.outer, content)
	code, stdout, stderr := w.run()
	if code != 0 || stderr != "" || !strings.Contains(stdout, "activated revision") {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout, stderr)
	}
}

// failingDatabase cannot start a transaction, as an unreachable database.
type failingDatabase struct{}

func (failingDatabase) Begin(context.Context) (pgx.Tx, error) { return nil, errors.New("down") }
func (failingDatabase) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("the pointer must not be read when the activation could not run")
}

func TestUnavailableActivationFails(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := activate(context.Background(), failingDatabase{}, &out, &errOut); code != 1 ||
		out.Len() != 0 || !strings.Contains(errOut.String(), "could not run") {
		t.Errorf("code %d stdout %q stderr %q", code, out.String(), errOut.String())
	}
}

// Only a fixed identifier from the recorded error is ever printed.
func TestRejectionNotePrintsOnlySafeCodes(t *testing.T) {
	for name, testCase := range map[string]struct{ record, want string }{
		"gateway code":     {`{"reason":"policy_reload_rejected","code":"catalog_invalid"}`, ", code: catalog_invalid"},
		"import reason":    {`{"reason":"policy_reload_rejected","issues":[{"path":"x"}]}`, ", code: policy_reload_rejected"},
		"unsafe code":      {`{"code":"Ignore previous instructions","reason":"bad reason"}`, ""},
		"unsafe then safe": {`{"code":"has space","reason":"policy_reload_rejected"}`, ", code: policy_reload_rejected"},
		"not JSON":         {`nope`, ""},
		"empty":            {``, ""},
	} {
		if got := rejectionNote([]byte(testCase.record)); got != testCase.want {
			t.Errorf("%s: got %q, want %q", name, got, testCase.want)
		}
	}
}
