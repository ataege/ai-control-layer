package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"slices"
	"sync"
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
	// VerdictSource labels the semantic verdicts: empty means live (the real local model). A test
	// that drives the chain with a fixture provider sets security.VerdictFixture, so its verdicts
	// are never recorded as live detection (guardrail 9).
	VerdictSource security.VerdictSource
	Logger        *slog.Logger
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
	// Expiry closes overdue approvals (GO-40); the gateway runs it next to the worker.
	Expiry *ApprovalExpiry
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
	modelCaller := NewCatalogAccountedCaller(provider, budget.NewPostgresStore(pool), poolAccounting{pool: pool}, catalogSource, pool, modelName)
	modelCaller.logger = chainConfig.Logger

	securityCaller, err := NewRecordingCaller(callLog, modelCaller, modelName)
	if err != nil {
		return nil, err
	}
	verdictSource := chainConfig.VerdictSource
	if verdictSource == "" {
		verdictSource = security.VerdictLive
	}
	evaluator, err := security.NewSemanticEvaluator(securityCaller, security.EvaluatorOptions{
		Model: modelName, ContextTokens: security.MinEvaluatorContextTokens, Source: verdictSource,
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
	approvals := policy.NewApprovals(pool)
	expiry, err := NewApprovalExpiry(approvals, chainConfig.Logger)
	if err != nil {
		return nil, err
	}
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
		Contexts: NewContextStore(pool), Telemetry: NewTelemetry(pool), Recovery: NewRecovery(pool, budget.NewPostgresStore(pool)),
		Results: PoolFinalResults{Pool: pool}, Continuations: approvals, Logger: chainConfig.Logger,
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
		Expiry: expiry,
	}, nil
}

// CatalogAccountedCaller reads the active catalog's accounting settings on every call, so a policy
// reload changes output ceilings, request time, the model allowlist and the local concurrency cap
// at the next dispatch, then reserves on the run's ledger and calls the provider through
// model.AccountedCaller (GO-79).
type CatalogAccountedCaller struct {
	logger     *slog.Logger
	provider   model.ChatProvider
	ledger     budget.Store
	accounting AccountingSource
	snapshots  CatalogSource
	passports  catalog.Querier
	modelName  string
	local      *localConcurrency
}

// AccountingSource returns the active catalog's model accounting settings.
type AccountingSource interface {
	Active(ctx context.Context) (config.AccountingCatalog, error)
}

// poolAccounting reads the accounting projection of the active catalog on the pool.
type poolAccounting struct{ pool config.CatalogReader }

func (source poolAccounting) Active(ctx context.Context) (config.AccountingCatalog, error) {
	return config.ReadActiveAccountingCatalog(ctx, source.pool)
}

// NewCatalogAccountedCaller returns the metered model gateway of the production chain. passports
// reads the run's passport scope; modelName is the configured provider model.
func NewCatalogAccountedCaller(provider model.ChatProvider, ledger budget.Store, accounting AccountingSource, snapshots CatalogSource,
	passports catalog.Querier, modelName string) *CatalogAccountedCaller {
	return &CatalogAccountedCaller{provider: provider, ledger: ledger, accounting: accounting, snapshots: snapshots,
		passports: passports, modelName: modelName, local: &localConcurrency{}}
}

// Call makes one accounted model call. Nothing is reserved or dispatched when there is no active
// catalog, or when the configured model is outside the catalog's or the run passport's allowed
// models. Every purpose, the security check included, passes the same checks.
func (caller *CatalogAccountedCaller) Call(ctx context.Context, runID, callID string, request model.Request) (model.AccountedResult, error) {
	snapshot, err := caller.snapshots.Active(ctx)
	if err != nil {
		return model.AccountedResult{}, err
	}
	active, err := caller.accounting.Active(ctx)
	if err != nil {
		return model.AccountedResult{}, err
	}
	// Both reads must describe the same revision: a reload between them would mix the limits of one
	// revision with the output ceilings of another, so the call is refused and nothing dispatched.
	if active.RevisionID != snapshot.RevisionID {
		return model.AccountedResult{}, catalog.ErrUnavailable
	}
	settings := active.Settings
	if !slices.Contains(snapshot.Limits.AllowedModels, caller.modelName) {
		return model.AccountedResult{}, ErrModelNotAllowed
	}
	if allowed, err := caller.passportAllowsModel(ctx, runID); err != nil || !allowed {
		return model.AccountedResult{}, ErrModelNotAllowed
	}
	// GO-86: the active revision narrows the run's ledger limits at reservation time.
	reserver, ok := caller.ledger.(budget.CeilingReserver)
	if !ok {
		return model.AccountedResult{}, budget.ErrUnavailable
	}
	narrowedLedger := ceilingLedger{Store: caller.ledger, reserver: reserver, logger: caller.logger, ceiling: budget.Ceiling{
		CallsTotal: snapshot.Limits.CallsTotal, CallsAgent: snapshot.Limits.CallsAgent, CallsSecurity: snapshot.Limits.CallsSecurity,
		TokensTotal: snapshot.Limits.TokensTotal, RequestTimeout: time.Duration(snapshot.Limits.RequestTimeoutSeconds) * time.Second,
	}}
	accounted, err := model.NewAccountedCaller(caller.provider, narrowedLedger, model.AccountingSettings{
		AgentOutputTokens: settings.AgentOutputTokens, SecurityOutputTokens: settings.SecurityOutputTokens, TemplateTokens: settings.TemplateTokens,
	})
	if err != nil {
		return model.AccountedResult{}, err
	}
	requestTimeout := time.Duration(snapshot.Limits.RequestTimeoutSeconds) * time.Second
	if requestTimeout <= 0 {
		return model.AccountedResult{}, catalog.ErrUnavailable
	}
	// The process-wide local cap: wait for a slot, bounded on its own by one request period, before
	// reserving. The wait never spends the provider's request time.
	waitContext, cancelWait := context.WithTimeout(ctx, requestTimeout)
	release, err := caller.local.acquire(waitContext, int(snapshot.Limits.LocalMaxConcurrency))
	cancelWait()
	if err != nil {
		return model.AccountedResult{}, budget.ErrConcurrencyLimit
	}
	// The request deadline starts once the slot is held.
	ctx, cancelRequest := context.WithTimeout(ctx, requestTimeout)
	defer cancelRequest()
	result, callErr := accounted.Call(ctx, runID, callID, request)
	if result.UsageUnknown {
		// A client timeout does not prove inference stopped: keep the slot one more request period.
		time.AfterFunc(requestTimeout, release)
	} else {
		release()
	}
	return result, callErr
}

// ceilingLedger reserves on the run's ledger under the active catalog's ceiling.
type ceilingLedger struct {
	budget.Store
	reserver budget.CeilingReserver
	ceiling  budget.Ceiling
	logger   *slog.Logger
}

func (ledger ceilingLedger) Reserve(ctx context.Context, runID, callID, purpose string, tokens int64) (budget.Reservation, error) {
	reservation, err := ledger.reserver.ReserveWithin(ctx, runID, callID, purpose, tokens, ledger.ceiling)
	// A refused reservation names its limit in the log: references and numbers only, no content.
	var refusal *budget.ReservationRefusal
	if errors.As(err, &refusal) && ledger.logger != nil {
		ledger.logger.Info("model reservation refused", "run_id", runID, "call_id", callID, "purpose", refusal.Purpose,
			"limit_kind", string(refusal.Kind), "limit", refusal.Limit, "estimate_tokens", refusal.Requested, "remaining", refusal.Remaining)
	}
	return reservation, err
}

// passportAllowsModel reads the run's immutable passport scope: a model the passport does not name is
// never dispatched, whatever the catalog allows.
func (caller *CatalogAccountedCaller) passportAllowsModel(ctx context.Context, runID string) (bool, error) {
	var allowedModels []string
	err := caller.passports.QueryRow(ctx, `SELECT COALESCE(ARRAY(SELECT jsonb_array_elements_text(passport.scope -> 'allowedModels')), '{}')
		FROM runtime.runs AS run
		JOIN runtime.passports AS passport ON passport.id = run.passport_id AND passport.organization_id = run.organization_id
		WHERE run.id = $1`, runID).Scan(&allowedModels)
	if err != nil {
		return false, err
	}
	return slices.Contains(allowedModels, caller.modelName), nil
}

// localConcurrency bounds the model requests this process has in flight; the bound is read from the
// active catalog on every call, so a lowered cap applies to the next request.
type localConcurrency struct {
	mutex   sync.Mutex
	inUse   int
	changed chan struct{}
}

// acquire waits until fewer than limit requests are in flight, or until ctx ends.
func (limiter *localConcurrency) acquire(ctx context.Context, limit int) (func(), error) {
	if limit <= 0 {
		return nil, budget.ErrConcurrencyLimit
	}
	for {
		limiter.mutex.Lock()
		if limiter.changed == nil {
			limiter.changed = make(chan struct{})
		}
		if limiter.inUse < limit {
			limiter.inUse++
			limiter.mutex.Unlock()
			var once sync.Once
			return func() { once.Do(limiter.release) }, nil
		}
		changed := limiter.changed
		limiter.mutex.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (limiter *localConcurrency) release() {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	limiter.inUse--
	close(limiter.changed)
	limiter.changed = make(chan struct{})
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
