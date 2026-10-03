// budgetcheck explicitly exercises provider accounting against an active central
// catalog and PostgreSQL. Synthetic diagnostic calls are not a governed task or
// evidence of semantic detection, identity or passport enforcement.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/database"
	"starter/services/gateway/internal/model"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if run(ctx, os.Stdout) != nil {
		_, _ = fmt.Fprintln(os.Stderr, "budget accounting diagnostic: FAIL")
		os.Exit(1)
	}
}

func run(ctx context.Context, output io.Writer) error {
	service, err := config.Load()
	if err != nil {
		return errors.New("diagnostic database configuration unavailable")
	}
	providerConfig, err := config.LoadModel()
	if err != nil {
		return errors.New("diagnostic model configuration unavailable")
	}
	pool, err := database.NewPool(ctx, database.Options{Host: service.Postgres.Host, Port: service.Postgres.Port,
		User: service.Postgres.User, Password: service.Postgres.Password, Database: service.Postgres.Database, ConnectTimeout: service.DatabaseTimeout})
	if err != nil {
		return errors.New("diagnostic database unavailable")
	}
	defer pool.Close()
	readContext, cancel := context.WithTimeout(ctx, service.DatabaseTimeout)
	catalog, err := config.ReadActiveAccountingCatalog(readContext, pool)
	cancel()
	if err != nil || !slices.Contains(catalog.Settings.AllowedModels, providerConfig.Name) {
		return errors.New("diagnostic active model catalog unavailable")
	}
	settings := catalog.Settings
	if settings.AgentOutputTokens > 4096 || settings.SecurityOutputTokens > 4096 {
		return errors.New("diagnostic output exceeds fixed diagnostic context")
	}
	provider, err := model.NewOllama(model.Options{BaseURL: providerConfig.BaseURL, Model: providerConfig.Name,
		Timeout: settings.RequestTimeout, MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		return errors.New("diagnostic provider unavailable")
	}
	store := budget.NewPostgresStore(pool)
	caller, err := model.NewAccountedCaller(provider, store, model.AccountingSettings{
		AgentOutputTokens: settings.AgentOutputTokens, SecurityOutputTokens: settings.SecurityOutputTokens, TemplateTokens: settings.TemplateTokens})
	if err != nil {
		return err
	}
	var identifier [16]byte
	if _, err = rand.Read(identifier[:]); err != nil {
		return errors.New("diagnostic identifier unavailable")
	}
	runID := "budgetcheck-" + hex.EncodeToString(identifier[:])
	writeContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = store.CreateRun(writeContext, runID, settings.TokensTotal)
	cancel()
	if err != nil {
		return err
	}
	for _, purpose := range []model.Purpose{model.AgentPurpose, model.SecurityPurpose} {
		result, err := caller.Call(ctx, runID, string(purpose), model.Request{Purpose: purpose, ContextTokens: 4096,
			Messages: []model.Message{{Role: "user", Content: "Synthetic accounting diagnostic. Reply OK. This is not a task or security assessment."}}})
		if err != nil {
			return err
		}
		if err = json.NewEncoder(output).Encode(struct {
			Label             string             `json:"label"`
			Purpose           model.Purpose      `json:"purpose"`
			RunID             string             `json:"run_id"`
			CatalogRevision   int64              `json:"catalog_revision"`
			ReservationTokens int64              `json:"reservation_tokens"`
			Settlement        *budget.Settlement `json:"settlement"`
		}{"synthetic budget accounting diagnostic", purpose, runID, catalog.RevisionID, result.ReservationTokens, result.Settlement}); err != nil {
			return err
		}
	}
	readContext, cancel = context.WithTimeout(ctx, service.DatabaseTimeout)
	snapshot, err := store.Snapshot(readContext, runID)
	cancel()
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Label  string          `json:"label"`
		Budget budget.Snapshot `json:"budget"`
	}{"synthetic budget accounting diagnostic", snapshot})
}
