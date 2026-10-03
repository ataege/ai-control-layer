package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/security"
	"starter/services/gateway/internal/tools"
	"starter/services/gateway/internal/worker"
)

// Provider limits of the production chain. The catalog's request timeout bounds each call inside
// providerTimeout; the byte limits bound one request and one response.
const (
	providerTimeout       = 2 * time.Minute
	providerRequestBytes  = 1 << 20
	providerResponseBytes = 1 << 20
)

// ErrModelNotConfigured is the answer of every model call when MODEL_NAME is not configured: the
// gateway still starts (health, admission), but no model request is ever dispatched.
var ErrModelNotConfigured = errors.New("model provider not configured")

// ChainConfig holds the explicit settings of the production chain.
type ChainConfig struct {
	// Model is the configured local model (config.LoadModel). Configured is false when MODEL_NAME
	// is missing or invalid; every model call then fails closed.
	Model           config.Model
	ModelConfigured bool
	Logger          *slog.Logger
}

// ProductionChain is the governed execution chain of the gateway process, built once. The judge
// evaluate endpoint (GO-82) reuses its gate, inspector and settings.
type ProductionChain struct {
	// ModelCaller is the metered model gateway: catalog accounting, the run ledger, the provider.
	ModelCaller ModelCaller
	// SecurityCaller records each security call before dispatch (RecordingCaller over ModelCaller).
	SecurityCaller *RecordingCaller
	Evaluator      *security.SemanticEvaluator
	Inspector      *security.Inspector
	Catalog        PoolCatalog
	Settings       *CatalogSettings
	Scopes         *policy.PassportScopeReader
	Gate           *policy.Gate
	Executor       *policy.Executor
	Loop           *Loop
	Worker         *worker.Service
}

// NewProductionChain builds the chain on the gateway pool and the process's catalog loader. It
// dispatches nothing and starts nothing; the caller starts Worker.
func NewProductionChain(pool *pgxpool.Pool, loader *catalog.Loader, chainConfig ChainConfig) (*ProductionChain, error) {
	if pool == nil || loader == nil || chainConfig.Logger == nil {
		return nil, ErrInvalid
	}
	modelName := chainConfig.Model.Name
	var provider model.ChatProvider = unconfiguredProvider{}
	if chainConfig.ModelConfigured {
		ollama, err := model.NewOllama(model.Options{BaseURL: chainConfig.Model.BaseURL, Model: modelName, Timeout: providerTimeout,
			MaxRequestBytes: providerRequestBytes, MaxResponseBytes: providerResponseBytes})
		if err != nil {
			return nil, err
		}
		provider = ollama
	} else {
		// A name that no catalog allows, so the allowlist also refuses before any dispatch.
		modelName = "unconfigured-model"
	}
	callLog := budget.NewCallLog(pool)
	catalogSource := PoolCatalog{Loader: loader, Pool: pool}
	modelCaller := &CatalogAccountedCaller{provider: provider, ledger: budget.NewPostgresStore(pool), catalog: pool}

	securityCaller, err := NewRecordingCaller(callLog, modelCaller, modelName)
	if err != nil {
		return nil, err
	}
	evaluator, err := security.NewSemanticEvaluator(securityCaller, security.EvaluatorOptions{
		Model: modelName, ContextTokens: security.MinEvaluatorContextTokens, Source: security.VerdictLive,
	})
	if err != nil {
		return nil, err
	}
	inspector := security.NewInspector(evaluator)
	settings := &CatalogSettings{catalog: catalogSource}
	scopes := policy.NewPassportScopeReader(pool)
	gate := policy.NewGate(scopes, policy.NewPostgresRecorder(pool), policy.NewPostgresRelationships(pool),
		policy.NewSecurityActionEvaluator(inspector, settings)).WithReviewFreezer(policy.NewPostgresReviewFreezer(pool))
	executor := policy.NewExecutor(pool, scopes, tools.Runner{})
	stepper, err := NewStepper(modelCaller, callLog, modelName)
	if err != nil {
		return nil, err
	}
	resultInspector, err := NewSecurityInspector(inspector)
	if err != nil {
		return nil, err
	}
	loop, err := NewLoop(LoopDependencies{
		Runs: repository.New(pool), Stepper: stepper, Gate: gate, Executor: executor, Inspector: resultInspector,
		Catalog: catalogSource, Scopes: scopes, Corrections: policy.NewCorrectionCounter(pool), Steps: budget.NewPostgresStore(pool),
		Contexts: NewContextStore(pool), Telemetry: NewTelemetry(pool), Logger: chainConfig.Logger,
	})
	if err != nil {
		return nil, err
	}
	workerID, err := newWorkerID()
	if err != nil {
		return nil, err
	}
	jobWorker, err := worker.New(worker.NewJobStore(pool), loop, worker.Options{
		WorkerID: workerID, Kinds: []string{contracts.JobKindAgentStep}, Logger: chainConfig.Logger,
	})
	if err != nil {
		return nil, err
	}
	service, err := worker.NewService(jobWorker)
	if err != nil {
		return nil, err
	}
	return &ProductionChain{
		ModelCaller: modelCaller, SecurityCaller: securityCaller, Evaluator: evaluator, Inspector: inspector,
		Catalog: catalogSource, Settings: settings, Scopes: scopes, Gate: gate, Executor: executor, Loop: loop, Worker: service,
	}, nil
}

// CatalogAccountedCaller reads the active catalog's accounting settings on every call, so a policy
// reload changes output ceilings and request time at the next dispatch, then reserves on the run's
// ledger and calls the provider through model.AccountedCaller.
type CatalogAccountedCaller struct {
	provider model.ChatProvider
	ledger   budget.Store
	catalog  config.CatalogReader
}

// Call makes one accounted model call; without an active catalog nothing is dispatched.
func (caller *CatalogAccountedCaller) Call(ctx context.Context, runID, callID string, request model.Request) (model.AccountedResult, error) {
	active, err := config.ReadActiveAccountingCatalog(ctx, caller.catalog)
	if err != nil {
		return model.AccountedResult{}, err
	}
	settings := active.Settings
	accounted, err := model.NewAccountedCaller(caller.provider, caller.ledger, model.AccountingSettings{
		AgentOutputTokens: settings.AgentOutputTokens, SecurityOutputTokens: settings.SecurityOutputTokens, TemplateTokens: settings.TemplateTokens,
	})
	if err != nil {
		return model.AccountedResult{}, err
	}
	if settings.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, settings.RequestTimeout)
		defer cancel()
	}
	return accounted.Call(ctx, runID, callID, request)
}

// CatalogSettings gives the security settings of the active catalog revision (policy's
// SecuritySettingsSource). A revision that is no longer active has no settings: the check is
// refused rather than run with other rules.
type CatalogSettings struct{ catalog CatalogSource }

// SettingsFor returns the settings when revisionID is the active revision.
func (source *CatalogSettings) SettingsFor(ctx context.Context, revisionID int64) (security.Settings, error) {
	snapshot, err := source.catalog.Active(ctx)
	if err != nil || snapshot.RevisionID != revisionID {
		return security.Settings{}, policy.ErrSecuritySettingsUnavailable
	}
	return snapshot.Security, nil
}

// unconfiguredProvider answers every request with ErrModelNotConfigured, dispatching nothing.
type unconfiguredProvider struct{}

func (unconfiguredProvider) Chat(context.Context, model.Request) (model.Result, error) {
	return model.Result{}, ErrModelNotConfigured
}

// newWorkerID names this process's worker for its lease tokens.
func newWorkerID() (string, error) {
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "gateway-" + hex.EncodeToString(random[:]), nil
}
