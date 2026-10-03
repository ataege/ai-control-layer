package testdb

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func mapLookup(values map[string]string) lookupFunc {
	return func(name string) (string, bool) { value, present := values[name]; return value, present }
}
func validSettings() map[string]string {
	return map[string]string{"POSTGRES_USER": " app user ", "POSTGRES_PASSWORD": " p@ss:/?#%=✓ ", "POSTGRES_DB": " app db "}
}
func TestReadOptionsPreservesCredentialsAndDefaults(t *testing.T) {
	options, configured, err := readOptions(mapLookup(validSettings()))
	if err != nil || !configured || options.Host != "localhost" || options.Port != 5432 || options.ConnectTimeout != connectionTimeout || options.User != " app user " || options.Password.Reveal() != " p@ss:/?#%=✓ " || options.Database != " app db " {
		t.Fatal("valid test settings were changed")
	}
}
func TestReadOptionsRejectsPartialAndMalformedSettings(t *testing.T) {
	for _, settings := range []map[string]string{{"TEST_DATABASE_REQUIRED": "1"}, {"POSTGRES_HOST": "host"}, {"POSTGRES_PASSWORD": "private-secret"}, {"POSTGRES_USER": " "}} {
		_, configured, err := readOptions(mapLookup(settings))
		if !configured || err == nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatal("partial configuration accepted or disclosed")
		}
	}
	for _, port := range []string{"", " ", "+5432", "-1", "0", "65536", "5432x", "999999999999999999999999"} {
		settings := validSettings()
		settings["POSTGRES_PORT"] = port
		_, _, err := readOptions(mapLookup(settings))
		if err == nil || !strings.Contains(err.Error(), "POSTGRES_PORT") {
			t.Fatal("invalid port accepted")
		}
	}
	settings := validSettings()
	settings["POSTGRES_HOST"] = " "
	if _, _, err := readOptions(mapLookup(settings)); err == nil {
		t.Fatal("blank explicit host accepted")
	}
	settings = validSettings()
	settings["POSTGRES_HOST"] = "127.0.0.1"
	settings["POSTGRES_PORT"] = "15432"
	options, _, err := readOptions(mapLookup(settings))
	if err != nil || options.Port != 15432 || options.Host != "127.0.0.1" {
		t.Fatal("custom endpoint lost")
	}
}
func TestReadOptionsIgnoresLegacyURLAndOptionalAbsent(t *testing.T) {
	for _, settings := range []map[string]string{{}, {"GATEWAY_TEST_DATABASE_URL": "postgres://private-secret@localhost/db"}, {"TEST_DATABASE_REQUIRED": "0"}} {
		_, configured, err := readOptions(mapLookup(settings))
		if configured || err != nil {
			t.Fatal("legacy URL became connection authority")
		}
	}
}
func TestConnectUnavailableOrCanceledIsBoundedAndSanitized(t *testing.T) {
	options, _, err := readOptions(mapLookup(validSettings()))
	if err != nil {
		t.Fatal(err)
	}
	options.Host = "127.0.0.1"
	options.Port = 1
	for _, canceled := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		if canceled {
			cancel()
		}
		started := time.Now()
		pool, err := connect(ctx, options)
		cancel()
		if pool != nil || err == nil || time.Since(started) > time.Second || strings.Contains(err.Error(), options.Password.Reveal()) || strings.Contains(err.Error(), options.User) {
			t.Fatal("unsafe or unbounded failed connection")
		}
	}
}
func TestIDCreatesDistinctUUIDv4(t *testing.T) {
	first, second := ID(t), ID(t)
	if first == second || len(first) != 36 || first[14] != '4' || !strings.ContainsRune("89ab", rune(first[19])) {
		t.Fatal("fixture identifier is not a distinct UUID v4")
	}
}

// This labelled wire-server double accepts the connection but never answers the
// PostgreSQL handshake, exercising timeout rather than immediate refusal.
func TestConnectStalledHandshakeHonorsDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("create stalled PostgreSQL wire fixture")
	}
	t.Cleanup(func() { _ = listener.Close() })
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = io.Copy(io.Discard, connection)
	}()
	options, _, err := readOptions(mapLookup(validSettings()))
	if err != nil {
		t.Fatal(err)
	}
	options.Host = "127.0.0.1"
	options.Port = listener.Addr().(*net.TCPAddr).Port
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	pool, err := connect(ctx, options)
	if pool != nil || err == nil || time.Since(started) > 2*time.Second || strings.Contains(err.Error(), options.Password.Reveal()) {
		t.Fatal("stalled PostgreSQL handshake did not fail safely within its deadline")
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("timed-out PostgreSQL fixture connection was not closed")
	}
}

func TestPostgresHarnessRoundTripAndRollback(t *testing.T) {
	pool := Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal("begin test database transaction")
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = transaction.Rollback(cleanupContext)
	})
	// A ledger row needs its run (alignment decision 1), so the round trip seeds a passport and run
	// in the same rolled-back transaction.
	organizationID, passportID, runID := ID(t), ID(t), ID(t)
	for _, statement := range []struct {
		sql       string
		arguments []any
	}{
		{`INSERT INTO runtime.passports(id, organization_id, actor_id, task_version, admission_catalog_revision_id, scope, limits, expires_at)
			VALUES ($1, $2, $3, 'go20_round_trip', 1, '{}', '{}', now() + interval '1 hour')`, []any{passportID, organizationID, ID(t)}},
		{`INSERT INTO runtime.runs(id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'queued')`, []any{runID, organizationID, passportID}},
		{`INSERT INTO runtime.model_token_budgets(run_id, organization_id, token_limit, call_limit, agent_call_limit, security_call_limit,
			request_timeout_ms, max_concurrent_calls) VALUES ($1, $2, 1000, 24, 12, 12, 20000, 2)`, []any{runID, organizationID}},
	} {
		if _, err = transaction.Exec(ctx, statement.sql, statement.arguments...); err != nil {
			t.Fatal("insert isolated runtime fixture; migrations must already be applied")
		}
	}
	var limit int64
	if err = transaction.QueryRow(ctx, "SELECT token_limit FROM runtime.model_token_budgets WHERE run_id=$1", runID).Scan(&limit); err != nil || limit != 1000 {
		t.Fatal("runtime fixture round trip failed")
	}
	if err = transaction.Rollback(ctx); err != nil {
		t.Fatal("rollback isolated runtime fixture")
	}
	if err = pool.QueryRow(ctx, "SELECT token_limit FROM runtime.model_token_budgets WHERE run_id=$1", runID).Scan(&limit); err != pgx.ErrNoRows {
		t.Fatal("rolled back fixture persisted")
	}
}
