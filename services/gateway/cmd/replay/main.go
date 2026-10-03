// replay submits one labelled replay of a hostile-note fixture (GO-05, GO-36) to a finished run,
// through the gateway's production gate. It is a deterministic rehearsal for the demonstration,
// never a model-generated action: the stored action and every event carry the replay label. It
// writes only what the gate writes and never executes the action.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"starter/services/gateway/internal/agent"
	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/database"
	"starter/services/gateway/internal/policy"
)

// Exit codes: the expected denial, an unexpected decision, and a replay that could not run.
const (
	exitExpectedDenial     = 0
	exitUnexpectedDecision = 1
	exitNotRun             = 2
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, arguments []string, output, errorOutput io.Writer) int {
	flags := flag.NewFlagSet("replay", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	runID := flags.String("run", "", "id of a finished run (completed, failed or stopped)")
	fixtureID := flags.String("fixture", "", "hostile-note fixture: "+strings.Join(policy.ReplayFixtureIDs(), ", "))
	if flags.Parse(arguments) != nil || *runID == "" || *fixtureID == "" || flags.NArg() > 0 {
		_, _ = fmt.Fprintln(errorOutput, "usage: replay -run <run id> -fixture <fixture id>")
		return exitNotRun
	}
	outcome, err := replay(ctx, *runID, *fixtureID, errorOutput)
	if err != nil {
		_, _ = fmt.Fprintln(errorOutput, "LABELLED REPLAY not run:", err)
		return exitNotRun
	}
	decision := outcome.Decision
	_, _ = fmt.Fprintf(output, "LABELLED REPLAY (deterministic rehearsal, not a model-generated action)\n"+
		"  label:     %s\n  run:       %s\n  action:    %s (step %d, tool %s)\n"+
		"  decision:  %s\n  reason:    %s\n  expected:  deny / %s\n",
		outcome.ReplaySource, outcome.Run.RunID, outcome.ActionID, outcome.StepNumber, outcome.Tool,
		decision.Outcome, decision.ReasonCode, outcome.ExpectedReason)
	if decision.Outcome != policy.OutcomeDeny || decision.ReasonCode != outcome.ExpectedReason {
		_, _ = fmt.Fprintln(output, "  result:    UNEXPECTED; the action was not executed")
		return exitUnexpectedDecision
	}
	_, _ = fmt.Fprintln(output, "  result:    denied by the production gate as its live equivalent would be; nothing executed")
	return exitExpectedDenial
}

// replay builds the gateway's production chain and submits the replay to its gate. The chain
// dispatches nothing and its worker is never started.
func replay(ctx context.Context, runID, fixtureID string, errorOutput io.Writer) (policy.ReplayOutcome, error) {
	service, err := config.Load()
	if err != nil {
		return policy.ReplayOutcome{}, errors.New("gateway configuration unavailable")
	}
	pool, err := database.NewPool(ctx, database.Options{Host: service.Postgres.Host, Port: service.Postgres.Port,
		User: service.Postgres.User, Password: service.Postgres.Password, Database: service.Postgres.Database, ConnectTimeout: service.DatabaseTimeout})
	if err != nil {
		return policy.ReplayOutcome{}, errors.New("database unavailable")
	}
	defer pool.Close()
	// Same as the gateway process: without a model configuration every model call fails closed.
	modelConfig, modelErr := config.LoadModel()
	chain, err := agent.NewProductionChain(pool, catalog.NewLoader(), agent.ChainConfig{
		Model: modelConfig, ModelConfigured: modelErr == nil, Logger: slog.New(slog.NewTextHandler(errorOutput, nil)),
	})
	if err != nil {
		return policy.ReplayOutcome{}, errors.New("production chain unavailable")
	}
	return policy.NewReplayRunner(pool, chain.Gate).Replay(ctx, runID, fixtureID)
}
