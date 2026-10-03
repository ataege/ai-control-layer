// Package httpserver wires routes, middleware and the server lifecycle.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"starter/services/gateway/internal/health"
	"starter/services/gateway/internal/logging"
)

// Options are the dependencies of the HTTP layer.
type Options struct {
	Logger       *slog.Logger
	Health       health.Handler
	ServiceToken logging.Secret
}

// NewHandler builds the full handler tree.
func NewHandler(options Options) http.Handler {
	mux := http.NewServeMux()

	// Method + exact path patterns. "GET" also matches HEAD.
	mux.HandleFunc("GET /health/live", options.Health.Live)
	mux.HandleFunc("GET /health/ready", options.Health.Ready)

	requireServiceToken := RequireServiceToken(options.ServiceToken)
	mux.Handle("GET /internal/ping", requireServiceToken(http.HandlerFunc(options.Health.Ping)))

	// RequestID is outermost so every log line and error body carries the id;
	// Recover is innermost so the access log records the 500 it writes.
	return Chain(withJSONErrors(mux),
		RequestID(options.Logger),
		AccessLog(options.Logger),
		Recover(options.Logger),
	)
}

// NewServer returns an http.Server with bounded timeouts and header size.
func NewServer(address string, handler http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
		// Route net/http's internal messages through slog as JSON.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
}

// Run serves until ctx is cancelled, then drains in-flight requests.
func Run(ctx context.Context, server *http.Server, shutdownTimeout time.Duration, logger *slog.Logger) error {
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}
	logger.Info("http server listening", "address", listener.Addr().String())

	// Request contexts descend from ctx, so a shutdown signal cancels in-flight
	// dependency calls (such as a readiness ping) instead of waiting them out.
	server.BaseContext = func(net.Listener) context.Context { return ctx }

	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()

	select {
	case err := <-serveErrors:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown requested", "cause", context.Cause(ctx).Error())
	}

	// A fresh context: the parent is already cancelled.
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()

	shutdownStartedAt := time.Now()
	if err := server.Shutdown(shutdownContext); err != nil {
		// Deadline hit: force-close the remaining connections.
		_ = server.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	// Serve returns ErrServerClosed after a clean Shutdown.
	if err := <-serveErrors; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	logger.Info("http server stopped", "drain_ms", time.Since(shutdownStartedAt).Milliseconds())
	return nil
}
