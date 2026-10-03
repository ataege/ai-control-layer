// catalogactivate runs the gateway's own catalog activation once (catalog.ActivateRequested): it
// validates the requested control-catalog revision with the gateway's parsers and, in one
// transaction, makes it active together with its signature feed, or records why it was rejected.
//
// A running gateway does this itself within seconds (catalog.WatchRequested). This command is for
// setups without a gateway, such as the test database and a freshly reset demo database, so the
// catalog is enforceable before the gateway starts. It is an explicit command, never run at
// startup, and it needs the same POSTGRES_* settings as the gateway (pnpm catalog:activate).
//
// Exit 0: a revision was activated, or nothing was requested and a revision is active. Exit 1:
// nothing is active, the requested revision was rejected (the safe rejection code is printed), or
// the activation could not run.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/database"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx))
}

func run(ctx context.Context) int {
	service, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "catalog activation: FAIL (the database configuration is unavailable)")
		return 1
	}
	pool, err := database.NewPool(ctx, database.Options{Host: service.Postgres.Host, Port: service.Postgres.Port,
		User: service.Postgres.User, Password: service.Postgres.Password, Database: service.Postgres.Database,
		ConnectTimeout: service.DatabaseTimeout})
	if err != nil {
		fmt.Fprintln(os.Stderr, "catalog activation: FAIL (the database is unavailable)")
		return 1
	}
	defer pool.Close()
	return activate(ctx, pool, os.Stdout, os.Stderr)
}
