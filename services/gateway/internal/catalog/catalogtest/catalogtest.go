// Package catalogtest activates an isolated, enforceable control catalog for database tests:
// the repository's policy as stored content, plus the real signature feed, inserted inside the
// test's own transaction so a rollback removes them.
package catalogtest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5"
)

// PolicyContent is config/policy.yaml as the importer stores it (control_catalog_revisions.content).
const PolicyContent = `{
	"schema_version": 1,
	"allowed_models": ["qwen3.5:4b"],
	"budgets": {"calls_total": 24, "calls_agent": 12, "calls_security": 12, "tokens_total": 20000,
		"agent_output_tokens": 512, "security_output_tokens": 256, "input_template_tokens": 1024,
		"request_timeout_seconds": 20, "local_max_concurrency": 2, "run_expiry_minutes": 15,
		"tool_attempts": 12, "corrections": 2},
	"controls": {
		"secret_pattern": {"enabled": true, "mode": "redact", "boundaries": ["model_input", "tool_result"]},
		"semantic_injection": {"enabled": true, "mode": "block", "threshold": 0.75,
			"boundaries": ["model_input", "tool_result", "action_proposal"]},
		"signature_match": {"enabled": true, "boundaries": ["model_input", "tool_result", "action_proposal"]}
	},
	"signatures": {"path": "attack-signatures.json", "revision": "feed_v2", "disabled_rules": []},
	"reports": {"enabled_templates": ["internal_investigation_v1", "vendor_reconciliation_v1"]}
}`

// InsertFeed stores the repository's config/attack-signatures.json as the trusted issuer's
// feed_v2 with its real digest (or reuses an imported one) and returns its id.
func InsertFeed(t *testing.T, transaction pgx.Tx) int64 {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	feedPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..", "config", "attack-signatures.json")
	feedBytes, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatalf("read the signature feed: %v", err)
	}
	digest := sha256.Sum256(feedBytes)
	// The trusted issuer and revision are unique; an imported row is reused as it is.
	if _, err := transaction.Exec(context.Background(), `INSERT INTO app.signature_feed_revisions
		(issuer, revision, source_file_name, source_text, file_digest, content, import_source)
		VALUES ('task-passport-security', 'feed_v2', 'attack-signatures.json', $1, $2, '{}', 'command')
		ON CONFLICT (issuer, revision) DO NOTHING`, string(feedBytes), hex.EncodeToString(digest[:])); err != nil {
		t.Fatalf("insert the signature feed: %v", err)
	}
	var feedID int64
	if err := transaction.QueryRow(context.Background(), `SELECT id FROM app.signature_feed_revisions
		WHERE issuer = 'task-passport-security' AND revision = 'feed_v2'`).Scan(&feedID); err != nil {
		t.Fatalf("read the signature feed: %v", err)
	}
	return feedID
}

// Activate stores content as a new catalog revision and points the active pointer at it and at
// feedID (nil for none). It returns the revision id.
func Activate(t *testing.T, transaction pgx.Tx, content string, feedID *int64) int64 {
	t.Helper()
	var revisionID int64
	err := transaction.QueryRow(context.Background(), `INSERT INTO app.control_catalog_revisions
		(schema_version, source_file_name, source_text, file_digest, content, import_source)
		VALUES (1, 'policy.yaml', 'catalogtest', repeat('c', 64), $1, 'command') RETURNING id`, content).Scan(&revisionID)
	if err != nil {
		t.Fatalf("insert the catalog revision: %v", err)
	}
	if _, err := transaction.Exec(context.Background(), `INSERT INTO app.control_catalog_pointer (id, active_revision_id, active_feed_revision_id)
		VALUES (1, $1, $2) ON CONFLICT (id) DO UPDATE
		SET active_revision_id = EXCLUDED.active_revision_id, active_feed_revision_id = EXCLUDED.active_feed_revision_id`,
		revisionID, feedID); err != nil {
		t.Fatalf("move the catalog pointer: %v", err)
	}
	return revisionID
}

// ActivatePolicy inserts the feed and activates PolicyContent with it; it returns the revision id.
func ActivatePolicy(t *testing.T, transaction pgx.Tx) int64 {
	t.Helper()
	feedID := InsertFeed(t, transaction)
	return Activate(t, transaction, PolicyContent, &feedID)
}

// Request stores content as a new catalog revision and sets only requested_revision_id, as the
// import does under the catalog activation protocol; the active revision is unchanged.
func Request(t *testing.T, transaction pgx.Tx, content string) int64 {
	t.Helper()
	var revisionID int64
	err := transaction.QueryRow(context.Background(), `INSERT INTO app.control_catalog_revisions
		(schema_version, source_file_name, source_text, file_digest, content, import_source)
		VALUES (1, 'policy.yaml', 'catalogtest', repeat('d', 64), $1, 'command') RETURNING id`, content).Scan(&revisionID)
	if err != nil {
		t.Fatalf("insert the requested revision: %v", err)
	}
	if _, err := transaction.Exec(context.Background(), `INSERT INTO app.control_catalog_pointer (id, requested_revision_id)
		VALUES (1, $1) ON CONFLICT (id) DO UPDATE SET requested_revision_id = EXCLUDED.requested_revision_id`, revisionID); err != nil {
		t.Fatalf("request the revision: %v", err)
	}
	return revisionID
}
