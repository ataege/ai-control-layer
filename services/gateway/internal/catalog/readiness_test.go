package catalog

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// "With no valid initial catalog, the gateway is not ready": readiness follows the same snapshot
// admission loads, so every case that issues no passport also reports not ready.
func TestPostgresReadinessFollowsTheEnforceableCatalog(t *testing.T) {
	world := newCatalogWorld(t)
	readiness := NewReadiness(NewLoader())
	ctx := context.Background()
	if readiness.Ready() {
		t.Fatal("ready before the first check")
	}
	world.activate(t, policyContent, &world.feedID)
	if !readiness.Check(ctx, world.outer) || !readiness.Ready() {
		t.Fatal("an enforceable catalog is not ready")
	}
	// Signature matching enabled without a bound feed: the catalog cannot be enforced.
	world.activate(t, policyContent, nil)
	if readiness.Check(ctx, world.outer) || readiness.Ready() {
		t.Fatal("a catalog without its feed is ready")
	}
	world.activate(t, policyContent, &world.feedID)
	if !readiness.Check(ctx, world.outer) {
		t.Fatal("readiness did not recover with the next good revision")
	}
	if _, err := world.outer.Exec(ctx, `UPDATE app.control_catalog_pointer SET active_revision_id = NULL WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if readiness.Check(ctx, world.outer) {
		t.Fatal("no active revision is ready")
	}
	if (*Readiness)(nil).Ready() || (*Readiness)(nil).Check(ctx, world.outer) {
		t.Error("a nil tracker must never be ready")
	}
}

// Watch names an unenforceable catalog once, not on every tick, and stops with its context.
func TestPostgresReadinessWatchLogsOnlyChanges(t *testing.T) {
	world := newCatalogWorld(t)
	world.activate(t, strings.Replace(policyContent, `"calls_agent": 12`, `"calls_agent": 30`, 1), &world.feedID)
	var logged strings.Builder
	logger := slog.New(slog.NewTextHandler(&logged, nil))
	readiness := NewReadiness(NewLoader())
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	readiness.Watch(ctx, world.outer, 10*time.Millisecond, logger)
	if count := strings.Count(logged.String(), "no enforceable control catalog is active"); count != 1 || readiness.Ready() {
		t.Errorf("warnings %d, ready %v; log:\n%s", count, readiness.Ready(), logged.String())
	}
}
