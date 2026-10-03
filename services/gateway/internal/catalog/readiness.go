package catalog

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// Readiness reports whether an enforceable catalog is active: "With no valid initial catalog, the
// gateway is not ready and cannot dispatch work" (report 1.2). It implements the health
// package's readiness reporter. Ready is false until the first check has loaded a snapshot.
type Readiness struct {
	loader *Loader
	ready  atomic.Bool
}

// NewReadiness returns a readiness tracker that loads the active snapshot through loader.
func NewReadiness(loader *Loader) *Readiness {
	return &Readiness{loader: loader}
}

// Ready reports the result of the latest check. A nil tracker is never ready.
func (readiness *Readiness) Ready() bool {
	return readiness != nil && readiness.ready.Load()
}

// Check loads the active snapshot once, as admission and the agent path do, and records whether
// it is enforceable: an active revision with valid limits and security settings, and the bound
// feed when signature matching is enabled. Any failure, the database included, is not ready.
func (readiness *Readiness) Check(ctx context.Context, querier Querier) bool {
	if readiness == nil {
		return false
	}
	_, err := readiness.loader.Active(ctx, querier)
	readiness.ready.Store(err == nil)
	return err == nil
}

// Watch checks at once and then every interval until ctx ends. It logs only when readiness
// changes, so an unenforceable catalog is named once rather than every tick.
func (readiness *Readiness) Watch(ctx context.Context, querier Querier, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	previous, first := false, true
	for {
		ready := readiness.Check(ctx, querier)
		if ctx.Err() == nil && (first || ready != previous) {
			if ready {
				logger.Info("an enforceable control catalog is active")
			} else {
				logger.Warn("no enforceable control catalog is active; the gateway is not ready")
			}
		}
		previous, first = ready, false
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
