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
}

// Live reports that the process is running; it never touches dependencies.
func (handler Handler) Live(responseWriter http.ResponseWriter, _ *http.Request) {
	WriteJSON(responseWriter, http.StatusOK, LivenessResponse{Status: StatusOK, Service: ServiceName})
}

// Ready reports whether PostgreSQL answers a ping within DatabaseTimeout.
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
	WriteJSON(responseWriter, http.StatusOK, ReadinessResponse{
		Status:  StatusOK,
		Service: ServiceName,
		Checks:  ReadinessChecks{Database: DependencyCheck{Status: CheckUp}},
	})
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
