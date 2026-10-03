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

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/database"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/repository"
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
	organizationID, runID, err := admitDiagnosticRun(ctx, pool, catalog, providerConfig.Name)
	if err != nil {
		return err
	}
	callLog := budget.NewCallLog(pool)
	for _, purpose := range []model.Purpose{model.AgentPurpose, model.SecurityPurpose} {
		callID, err := callLog.RecordDispatch(ctx, organizationID, runID, string(purpose), providerConfig.Name)
		if err != nil {
			return err
		}
		result, err := caller.Call(ctx, runID, callID, model.Request{Purpose: purpose, ContextTokens: 4096,
			Messages: []model.Message{{Role: "user", Content: "Synthetic accounting diagnostic. Reply OK. This is not a task or security assessment."}}})
		outcome := budget.CallCompleted
		if err != nil {
			outcome = budget.CallFailed
			if result.UsageUnknown {
				outcome = budget.CallUsageUnknown
			}
		}
		recordContext, cancelRecord := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		_ = callLog.RecordOutcome(recordContext, organizationID, callID, outcome)
		cancelRecord()
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

// diagnosticJobKind is a job kind no worker claims: the diagnostic run is never stepped.
const diagnosticJobKind = "budgetcheck_diagnostic"

// admitDiagnosticRun stores a labelled synthetic passport, run, unclaimed job and open ledger in one
// transaction, with one agent and one security call allowed. The rows stay for inspection.
func admitDiagnosticRun(ctx context.Context, pool *pgxpool.Pool, catalog config.AccountingCatalog, modelName string) (organizationID, runID string, err error) {
	identifiers := make([]string, 5)
	for index := range identifiers {
		var random [16]byte
		if _, err = rand.Read(random[:]); err != nil {
			return "", "", errors.New("diagnostic identifier unavailable")
		}
		random[6] = (random[6] & 0x0f) | 0x40
		random[8] = (random[8] & 0x3f) | 0x80
		encoded := hex.EncodeToString(random[:])
		identifiers[index] = encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
	}
	issuedAt := time.Now().UTC()
	timeoutSeconds := max(int64(catalog.Settings.RequestTimeout/time.Second), 1)
	passport := contracts.Passport{
		PassportID: identifiers[0], RunID: identifiers[1], OrganizationID: identifiers[2], ActorID: identifiers[3],
		TaskVersion: "budgetcheck_diagnostic", AdmissionCatalogRevisionID: catalog.RevisionID,
		IssuedAt: issuedAt, ExpiresAt: issuedAt.Add(15 * time.Minute),
		Scope: contracts.PassportScope{AllowedModels: []string{modelName}},
		Limits: contracts.PassportLimits{CallsTotal: 2, CallsAgent: 1, CallsSecurity: 1, TokensTotal: catalog.Settings.TokensTotal,
			RequestTimeoutSeconds: timeoutSeconds, LocalMaxConcurrency: 1, RunExpiryMinutes: 15},
	}
	writeContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = repository.New(pool).InTransaction(writeContext, func(transaction repository.Tx) error {
		if err := transaction.InsertAdmission(writeContext, passport, repository.NewJob{ID: identifiers[4], Kind: diagnosticJobKind}); err != nil {
			return err
		}
		return budget.OpenRunLedger(writeContext, transaction.Raw(), passport.OrganizationID, passport.RunID, passport.Limits)
	})
	if err != nil {
		return "", "", errors.New("diagnostic run could not be admitted")
	}
	return passport.OrganizationID, passport.RunID, nil
}
