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

	"starter/services/gateway/internal/admission"
	"starter/services/gateway/internal/agent"
	"starter/services/gateway/internal/api"
	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/database"
	"starter/services/gateway/internal/evaluation"
	"starter/services/gateway/internal/health"
	"starter/services/gateway/internal/httpserver"
	"starter/services/gateway/internal/logging"
	"starter/services/gateway/internal/operatorcontext"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/repository"
)

const (
	// Kept below the default 10 s container stop grace period.
	shutdownTimeout = 8 * time.Second
	// The worker drains in parallel with HTTP, inside the same budget.
	workerDrainTimeout = 6 * time.Second
	healthcheckTimeout = 2 * time.Second
	// A requested catalog revision is validated within about this long after its import.
	catalogActivationInterval = time.Second
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

	// Validates and activates each newly requested catalog revision (catalog activation
	// protocol). Stopped before the pool closes: the deferred calls run in reverse order.
	activationStopped := make(chan struct{})
	go func() {
		defer close(activationStopped)
		catalog.WatchRequested(signalContext, pool, catalogActivationInterval, logger)
	}()
	defer func() {
		stopSignals()
		<-activationStopped
	}()

	operatorContextVerifier, err := operatorcontext.NewVerifier(loadedConfig.OperatorContextSigningKey)
	if err != nil {
		return err
	}

	// One catalog loader for admission and the agent chain. A missing model configuration does
	// not stop the gateway: every model call then fails closed and no run can take a step.
	catalogLoader := catalog.NewLoader()
	// Readiness follows the active catalog (GO-72): no enforceable catalog, not ready. Stopped
	// before the pool closes, like the activation watcher.
	catalogReadiness := catalog.NewReadiness(catalogLoader)
	readinessStopped := make(chan struct{})
	go func() {
		defer close(readinessStopped)
		catalogReadiness.Watch(signalContext, pool, catalogActivationInterval, logger)
	}()
	defer func() {
		stopSignals()
		<-readinessStopped
	}()
	modelConfig, modelErr := config.LoadModel()
	if modelErr != nil {
		logger.Warn("model not configured; every model call fails closed", "error", modelErr.Error())
	}
	chain, err := agent.NewProductionChain(pool, catalogLoader, agent.ChainConfig{
		Model: modelConfig, ModelConfigured: modelErr == nil, Logger: logger,
	})
	if err != nil {
		return err
	}
	chain.Worker.Start()
	// Overdue approvals close as expired while no worker holds their run (GO-40).
	go chain.Expiry.Run(signalContext)
	workerStopped := make(chan struct{})
	go func() {
		defer close(workerStopped)
		<-signalContext.Done()
		if stopErr := chain.Worker.Stop(workerDrainTimeout); stopErr != nil {
			logger.Warn("worker stopped before its step finished; the job is reclaimed after its lease", "error", stopErr.Error())
		}
	}()

	runtimeRepository := repository.New(pool)
	handler := httpserver.NewHandler(httpserver.Options{
		Logger: logger,
		Health: health.Handler{Database: pool, DatabaseTimeout: loadedConfig.DatabaseTimeout, Logger: logger,
			Worker: chain.Worker, Catalog: catalogReadiness},
		ServiceToken:    loadedConfig.ServiceToken,
		OperatorContext: operatorContextVerifier,
		InternalCommands: api.Commands(api.Dependencies{
			Admitter:  admission.New(runtimeRepository, catalogLoader),
			Options:   admission.NewOptionsReader(pool, catalogLoader),
			Canceller: runtimeRepository,
			Approvals: policy.NewApprovals(pool),
			Runs:      runtimeRepository,
			Database:  pool,
			// GO-82: the same controls as the agent path, on the chain built above.
			Evaluator: evaluation.New(evaluation.Dependencies{
				Repository: runtimeRepository, Catalog: catalogLoader, Database: pool,
				Semantic: chain.Evaluator, Inspector: chain.Inspector,
				Gates: evaluation.JudgeGates(chain.Scopes, policy.NewPostgresRelationships(pool),
					policy.NewSecurityActionEvaluator(chain.Inspector, chain.Settings)),
			}),
		}),
	})
	listenAddress := net.JoinHostPort(loadedConfig.Host, strconv.Itoa(loadedConfig.Port))
	server := httpserver.NewServer(listenAddress, handler, logger)
	runErr := httpserver.Run(signalContext, server, shutdownTimeout, logger)
	// The worker stops before the deferred pool.Close; stopSignals also ends a run that returned
	// without a signal (for example a listen failure).
	stopSignals()
	<-workerStopped
	return runErr
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
