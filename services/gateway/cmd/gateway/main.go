// Command gateway is the internal Go service of the starter.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/database"
	"starter/services/gateway/internal/health"
	"starter/services/gateway/internal/httpserver"
	"starter/services/gateway/internal/logging"
)

const (
	// Kept below the default 10 s container stop grace period.
	shutdownTimeout    = 8 * time.Second
	healthcheckTimeout = 2 * time.Second
)

func main() {
	healthcheckMode := flag.Bool("healthcheck", false, "probe this service's own /health/live and exit 0 or 1")
	flag.Parse()

	if *healthcheckMode {
		os.Exit(runHealthcheck())
	}

	// JSON from the first line, so configuration errors are machine-readable too.
	slog.SetDefault(logging.New(os.Stdout, slog.LevelInfo))
	if err := run(); err != nil {
		slog.Error("gateway stopped with an error", "error", err.Error())
		os.Exit(1)
	}
}

func run() error {
	loadedConfig, err := config.Load()
	if err != nil {
		return err
	}
	logger := logging.New(os.Stdout, loadedConfig.LogLevel)
	slog.SetDefault(logger)

	// Cancelled on SIGINT (Ctrl+C) or SIGTERM (container stop).
	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// Background context: the pool outlives startup and is ended by Close.
	pool, err := database.NewPool(context.Background(), database.Options{
		Host:           loadedConfig.Postgres.Host,
		Port:           loadedConfig.Postgres.Port,
		User:           loadedConfig.Postgres.User,
		Password:       loadedConfig.Postgres.Password,
		Database:       loadedConfig.Postgres.Database,
		ConnectTimeout: loadedConfig.DatabaseTimeout,
	})
	if err != nil {
		return err
	}
	// Runs after Run returns: HTTP drains first, then the pool closes.
	defer pool.Close()

	handler := httpserver.NewHandler(httpserver.Options{
		Logger:       logger,
		Health:       health.Handler{Database: pool, DatabaseTimeout: loadedConfig.DatabaseTimeout, Logger: logger},
		ServiceToken: loadedConfig.ServiceToken,
	})
	listenAddress := net.JoinHostPort(loadedConfig.Host, strconv.Itoa(loadedConfig.Port))
	server := httpserver.NewServer(listenAddress, handler, logger)
	return httpserver.Run(signalContext, server, shutdownTimeout, logger)
}

// runHealthcheck lets a container health check work without curl or wget.
// It reads only the bind variables, so it needs no secrets.
// Values are trimmed exactly like the server's own configuration.
func runHealthcheck() int {
	probeHost := strings.TrimSpace(os.Getenv("GATEWAY_HOST"))
	// A wildcard bind address is reached through loopback.
	if probeHost == "" || probeHost == "0.0.0.0" || probeHost == "::" {
		probeHost = "127.0.0.1"
	}
	probePort := strings.TrimSpace(os.Getenv("GATEWAY_PORT"))
	if probePort == "" {
		probePort = "8080"
	}

	client := &http.Client{Timeout: healthcheckTimeout}
	response, err := client.Get("http://" + net.JoinHostPort(probeHost, probePort) + "/health/live")
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck failed: liveness endpoint unreachable")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck failed: unexpected status", response.StatusCode)
		return 1
	}
	return 0
}
