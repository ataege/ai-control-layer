// Package health serves the liveness, readiness and internal ping endpoints.
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"starter/services/gateway/internal/logging"
)

// databaseUnreachableMessage is the only failure text clients ever see.
const databaseUnreachableMessage = "database unreachable"

// ReadinessReporter is a background component readiness covers, such as the worker loop
// (decision 5). The readiness schema has no field for it, so a component that is not ready
// makes the whole response unavailable while the database check still reports its real result.
type ReadinessReporter interface {
	Ready() bool
}

// Pinger is the part of *pgxpool.Pool that readiness needs.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler serves the probe endpoints.
type Handler struct {
	Database Pinger
	// DatabaseTimeout bounds one readiness ping.
	DatabaseTimeout time.Duration
	Logger          *slog.Logger
	// Worker is nil when the process runs no worker loop.
	Worker ReadinessReporter
}

// Live reports that the process is running; it never touches dependencies.
func (handler Handler) Live(responseWriter http.ResponseWriter, _ *http.Request) {
	WriteJSON(responseWriter, http.StatusOK, LivenessResponse{Status: StatusOK, Service: ServiceName})
}

// Ready reports whether PostgreSQL answers a ping within DatabaseTimeout and, when a worker is
// configured, whether its loop is running.
func (handler Handler) Ready(responseWriter http.ResponseWriter, request *http.Request) {
	pingContext, cancelPing := context.WithTimeout(request.Context(), handler.DatabaseTimeout)
	defer cancelPing()

	if err := handler.Database.Ping(pingContext); err != nil {
		// Driver errors name the host and user: server log only, never the response.
		logging.FromContext(request.Context(), handler.Logger).Warn("readiness check failed",
			"check", "database", "error", err.Error())
		WriteJSON(responseWriter, http.StatusServiceUnavailable, ReadinessResponse{
			Status:  StatusUnavailable,
			Service: ServiceName,
			Checks:  ReadinessChecks{Database: DependencyCheck{Status: CheckDown, Message: databaseUnreachableMessage}},
		})
		return
	}
	databaseUp := ReadinessChecks{Database: DependencyCheck{Status: CheckUp}}
	if handler.Worker != nil && !handler.Worker.Ready() {
		logging.FromContext(request.Context(), handler.Logger).Warn("readiness check failed",
			"check", "worker", "error", "worker loop not running")
		WriteJSON(responseWriter, http.StatusServiceUnavailable, ReadinessResponse{
			Status: StatusUnavailable, Service: ServiceName, Checks: databaseUp,
		})
		return
	}
	WriteJSON(responseWriter, http.StatusOK, ReadinessResponse{Status: StatusOK, Service: ServiceName, Checks: databaseUp})
}

// Ping proves authenticated reachability only; it does not touch the database.
func (handler Handler) Ping(responseWriter http.ResponseWriter, _ *http.Request) {
	WriteJSON(responseWriter, http.StatusOK, GatewayPingResponse{Status: StatusOK, Service: ServiceName})
}

// WriteJSON sends a JSON body that must never be cached.
func WriteJSON(responseWriter http.ResponseWriter, statusCode int, body any) {
	responseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	responseWriter.Header().Set("Cache-Control", "no-store")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(body)
}
