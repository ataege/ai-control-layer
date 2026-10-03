// benchmark is the repeatable performance benchmark (GO-81). It measures one permitted operation,
// the governed tool-result path of a read_invoice result: the policy lookup (active catalog and
// security settings from PostgreSQL) and the hybrid inspection, with the semantic check off, on
// with a fixture caller (gateway overhead without a model), and on with the live local model
// (--live). It writes no row. The passport, the gate, the effect and its commit are outside the
// measured operation and unchanged; the in-process settings changes below never touch the catalog.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/database"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/security"
	"starter/services/gateway/internal/tools"
)

// Phase names follow runtime.timing_records (GO-80).
const (
	phasePolicyLookup  = "policy_lookup"
	phaseDeterministic = "deterministic"
	phaseSemantic      = "semantic"
	phaseProvider      = "provider"
	phaseTotal         = "total"
)

// fixtureVerdict is the fixture caller's answer: a valid, low-risk verdict, so every sample takes
// the permitted path rather than the failure path.
const fixtureVerdict = `{"risk_category":"none","score":0.02,"reason_code":"no_risk_found"}`

// benchmarkRunID labels the in-memory inspection input; no run row exists or is written.
const benchmarkRunID = "benchmark-run"

type options struct {
	samples, warmup         int
	live                    bool
	liveSamples, liveWarmup int
	outputPath              string
}

// Report is the benchmark output.
type Report struct {
	Kind            string                `json:"kind"`
	Method          []string              `json:"method"`
	Environment     Environment           `json:"environment"`
	Workload        Workload              `json:"workload"`
	Configurations  []ConfigurationResult `json:"configurations"`
	RecordedTimings RecordedTimings       `json:"recordedTimings"`
	Limitations     []string              `json:"limitations"`
}

// Environment is what the results depend on.
type Environment struct {
	StartedAt       time.Time `json:"startedAt"`
	GitCommit       string    `json:"gitCommit"`
	GitDirty        *bool     `json:"gitDirty"`
	GoVersion       string    `json:"goVersion"`
	OperatingSystem string    `json:"operatingSystem"`
	Architecture    string    `json:"architecture"`
	CPUCount        int       `json:"cpuCount"`
	CPUModel        string    `json:"cpuModel"`
	// LoadAverage is the system load (1, 5 and 15 minutes) when the run started; other work on
	// the machine changes every latency below.
	LoadAverage             string  `json:"loadAverage"`
	Database                string  `json:"database"`
	ActiveCatalogRevisionID int64   `json:"activeCatalogRevisionId"`
	FeedRevision            *string `json:"feedRevision"`
	Concurrency             int     `json:"concurrency"`
}

// Workload describes the measured operation and its payload.
type Workload struct {
	Operation          string `json:"operation"`
	Tool               string `json:"tool"`
	Fixture            string `json:"fixture"`
	PayloadBytes       int    `json:"payloadBytes"`
	UntrustedNoteBytes int    `json:"untrustedNoteBytes"`
}

// ConfigurationResult is one configuration's measurements. Errored samples are counted and left
// out of the distributions.
type ConfigurationResult struct {
	Name                string                  `json:"name"`
	Description         string                  `json:"description"`
	SemanticCheck       string                  `json:"semanticCheck"`
	Model               *string                 `json:"model"`
	Warmup              int                     `json:"warmup"`
	Samples             int                     `json:"samples"`
	Errors              int                     `json:"errors"`
	Outcomes            map[string]int          `json:"outcomes"`
	WallSeconds         float64                 `json:"wallSeconds"`
	ThroughputPerSecond *float64                `json:"throughputPerSecond"`
	Phases              map[string]Distribution `json:"phases"`
	// GatewayOverhead is the total minus the provider-reported model time.
	GatewayOverhead Distribution `json:"gatewayOverhead"`
}

// RecordedTimings aggregates what the gateway itself recorded in runtime.timing_records during
// real runs, separately from this benchmark's own measurements.
type RecordedTimings struct {
	Source string        `json:"source"`
	Phases []PhaseRecord `json:"phases"`
}

// PhaseRecord is one phase of runtime.timing_records.
type PhaseRecord struct {
	Phase  string       `json:"phase"`
	Failed int          `json:"failed"`
	Spans  Distribution `json:"spans"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	settings := options{}
	flags := flag.NewFlagSet("benchmark", flag.ContinueOnError)
	flags.IntVar(&settings.samples, "samples", 300, "measured samples per configuration without a live model")
	flags.IntVar(&settings.warmup, "warmup", 20, "unmeasured warmup samples per configuration without a live model")
	flags.BoolVar(&settings.live, "live", false, "also measure the semantic check with the live local model (MODEL_NAME)")
	flags.IntVar(&settings.liveSamples, "live-samples", 10, "measured samples with the live model")
	flags.IntVar(&settings.liveWarmup, "live-warmup", 1, "unmeasured warmup samples with the live model")
	flags.StringVar(&settings.outputPath, "out", "", "also write the JSON report to this file")
	if flags.Parse(os.Args[1:]) != nil || settings.samples < 1 || settings.warmup < 0 || settings.liveSamples < 1 || settings.liveWarmup < 0 {
		os.Exit(2)
	}
	if err := run(ctx, settings, os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "benchmark: FAIL: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, settings options, output io.Writer) error {
	service, err := config.Load()
	if err != nil {
		return errors.New("database configuration unavailable; run `pnpm run setup`, `pnpm db:migration:run` and `pnpm db:roles`")
	}
	pool, err := database.NewPool(ctx, database.Options{Host: service.Postgres.Host, Port: service.Postgres.Port,
		User: service.Postgres.User, Password: service.Postgres.Password, Database: service.Postgres.Database, ConnectTimeout: service.DatabaseTimeout})
	if err != nil {
		return errors.New("database unavailable")
	}
	defer pool.Close()

	lookup := newPolicyLookup(pool)
	activeSettings, accounting, err := lookup.load(ctx)
	if err != nil {
		return errors.New("no enforceable active control catalog; run `pnpm policy:import` (or `pnpm db:seed`) first")
	}
	input, workload, err := buildWorkload()
	if err != nil {
		return err
	}
	report := Report{
		Kind: "task-passport-benchmark",
		Method: []string{
			"Monotonic durations per sample, one sample at a time (concurrency 1), after unmeasured warmup samples; " +
				"the configurations without a live model are interleaved sample by sample.",
			"policy_lookup: read the active catalog revision and its security settings from PostgreSQL, as each inspection does.",
			"deterministic: the inspection minus the semantic evaluator; semantic: the evaluator including its model call; " +
				"provider: the model time the provider reports; total: policy_lookup plus the inspection.",
			"Percentiles are nearest-rank over the measured samples; a statistic without observations is null.",
			"Semantic on and off are derived in process from the active revision's settings; the catalog itself is not changed.",
		},
		Environment: describeEnvironment(service, activeSettings),
		Workload:    workload,
		Limitations: []string{
			"The passport, gate, tool effect and commit are not inside the measured operation.",
			"The fixture configuration answers instantly: it measures gateway overhead, never detection quality or model latency.",
			"Results describe this machine and this run only.",
		},
	}

	fixtureEvaluator, err := security.NewSemanticEvaluator(fixtureCaller{}, security.EvaluatorOptions{
		Model: "fixture", ContextTokens: security.MinEvaluatorContextTokens, Source: security.VerdictFixture})
	if err != nil {
		return err
	}
	fixtureInspector := security.NewInspector(fixtureEvaluator)
	report.Configurations = measure(ctx, []configuration{
		{name: "semantic_off", description: "Deterministic controls only: the semantic check is disabled in process.",
			semanticCheck: "off", inspector: fixtureInspector, configure: withSemantic(false)},
		{name: "semantic_on_fixture", description: "Semantic check on, answered by a labelled fixture caller with no model: gateway overhead.",
			semanticCheck: "fixture", inspector: fixtureInspector, configure: withSemantic(true)},
	}, settings.warmup, settings.samples, lookup, input)

	if settings.live {
		liveInspector, modelName, err := newLiveInspector(accounting)
		if err != nil {
			return err
		}
		report.Configurations = append(report.Configurations, measure(ctx, []configuration{
			{name: "semantic_on_live", description: "Semantic check on with the live local model: actual semantic and provider delay.",
				semanticCheck: "live", model: &modelName, inspector: liveInspector, configure: withSemantic(true)},
		}, settings.liveWarmup, settings.liveSamples, lookup, input)...)
		report.Limitations = append(report.Limitations,
			"The live run reserves tokens in an in-memory ledger, not against any run's allowance.")
	} else {
		report.Limitations = append(report.Limitations, "No live model run: pass --live to measure actual semantic and provider delay.")
	}

	if report.RecordedTimings, err = readRecordedTimings(ctx, pool); err != nil {
		return errors.New("timing records unavailable")
	}
	return writeReport(report, settings.outputPath, output)
}

// policyLookup reads the active catalog and its security settings the way the gateway does.
type policyLookup struct {
	pool     *pgxpool.Pool
	settings *policy.CatalogSecuritySettings
}

func newPolicyLookup(pool *pgxpool.Pool) policyLookup {
	return policyLookup{pool: pool, settings: policy.NewCatalogSecuritySettings(pool)}
}

func (lookup policyLookup) load(ctx context.Context) (security.Settings, config.AccountingCatalog, error) {
	readContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	catalog, err := config.ReadActiveAccountingCatalog(readContext, lookup.pool)
	if err != nil {
		return security.Settings{}, config.AccountingCatalog{}, err
	}
	settings, err := lookup.settings.SettingsFor(readContext, catalog.RevisionID)
	return settings, catalog, err
}

// withSemantic derives the measured configuration from the active settings: the semantic check
// on or off for tool results. Every other control stays as the catalog sets it.
func withSemantic(enabled bool) func(security.Settings) security.Settings {
	return func(settings security.Settings) security.Settings {
		settings.SemanticInjection.Enabled = enabled
		boundaries := slices.Clone(settings.SemanticInjection.Boundaries)
		if enabled && !slices.Contains(boundaries, security.BoundaryToolResult) {
			boundaries = append(boundaries, security.BoundaryToolResult)
		}
		settings.SemanticInjection.Boundaries = boundaries
		return settings
	}
}

// configuration is one measured way to run the operation.
type configuration struct {
	name, description, semanticCheck string
	model                            *string
	inspector                        *security.Inspector
	configure                        func(security.Settings) security.Settings
}

// measure runs the warmup, then the measured samples of the configurations interleaved one
// sample at a time, so changing load on the machine affects every configuration alike. A
// configuration's time is the sum of its own samples; its throughput is samples per that time.
func measure(ctx context.Context, configurations []configuration, warmup, samples int, lookup policyLookup,
	input security.ToolResultInput) []ConfigurationResult {
	results := make([]ConfigurationResult, len(configurations))
	observations := make([]map[string][]time.Duration, len(configurations))
	overhead := make([][]time.Duration, len(configurations))
	busy := make([]time.Duration, len(configurations))
	for index, current := range configurations {
		results[index] = ConfigurationResult{Name: current.name, Description: current.description, SemanticCheck: current.semanticCheck,
			Model: current.model, Warmup: warmup, Samples: samples, Outcomes: map[string]int{}, Phases: map[string]Distribution{}}
		observations[index] = map[string][]time.Duration{}
	}
	for range warmup {
		for _, current := range configurations {
			_, _ = sample(ctx, lookup, current.inspector, input, current.configure)
		}
	}
	for range samples {
		for index, current := range configurations {
			started := time.Now()
			measured, outcome := sample(ctx, lookup, current.inspector, input, current.configure)
			busy[index] += time.Since(started)
			results[index].Outcomes[outcome]++
			if measured == nil {
				results[index].Errors++
				continue
			}
			for phase, duration := range measured.phases {
				observations[index][phase] = append(observations[index][phase], duration)
			}
			overhead[index] = append(overhead[index], measured.phases[phaseTotal]-measured.phases[phaseProvider])
		}
	}
	for index := range results {
		result := &results[index]
		result.WallSeconds = busy[index].Seconds()
		if measuredSamples := samples - result.Errors; measuredSamples > 0 && result.WallSeconds > 0 {
			throughput := float64(measuredSamples) / result.WallSeconds
			result.ThroughputPerSecond = &throughput
		}
		for _, phase := range []string{phasePolicyLookup, phaseDeterministic, phaseSemantic, phaseProvider, phaseTotal} {
			result.Phases[phase] = summarize(observations[index][phase])
		}
		result.GatewayOverhead = summarize(overhead[index])
	}
	return results
}

type sampleTimings struct{ phases map[string]time.Duration }

// sample measures one policy lookup and inspection. It returns nil timings for an errored or
// paused sample, with the outcome name.
func sample(ctx context.Context, lookup policyLookup, inspector *security.Inspector, input security.ToolResultInput,
	configure func(security.Settings) security.Settings) (*sampleTimings, string) {
	started := time.Now()
	settings, _, err := lookup.load(ctx)
	looked := time.Now()
	if err != nil {
		return nil, "policy_lookup_error"
	}
	inspection, err := inspector.InspectToolResult(ctx, input, configure(settings))
	finished := time.Now()
	if err != nil || inspection.Outcome == security.ResultPaused {
		return nil, string(security.ResultPaused)
	}
	var semantic, provider time.Duration
	for _, call := range inspection.SemanticCalls {
		semantic += call.Record.Duration
		provider += call.ProviderDuration
	}
	timings := &sampleTimings{phases: map[string]time.Duration{
		phasePolicyLookup:  looked.Sub(started),
		phaseDeterministic: finished.Sub(looked) - semantic,
		phaseTotal:         finished.Sub(started),
	}}
	if len(inspection.SemanticCalls) > 0 {
		timings.phases[phaseSemantic] = semantic
	}
	if provider > 0 {
		timings.phases[phaseProvider] = provider
	}
	return timings, string(inspection.Outcome)
}

// buildWorkload shapes one permitted read_invoice result exactly as the worker inspects it: the
// minimized model-facing result, with the internal note as the untrusted, semantically checked
// path (agent.SecurityInspector).
func buildWorkload() (security.ToolResultInput, Workload, error) {
	note := "Investigation note: INV104 appears twice for Atlas with the same total and due date. " +
		"Confirm with accounts payable which booking is the duplicate before any vendor contact."
	invoice := tools.InvoiceResult{
		InvoiceID: "invoice_A01", Version: 1, VendorID: "vendor_Atlas", ExternalReference: "INV104",
		Currency: "EUR", TotalMinorUnits: 125000, IssuedOn: "2026-09-01", DueOn: "2026-10-31",
		InternalNote: &tools.InvoiceNote{Text: note, Classification: "internal_only"},
	}
	minimized, err := tools.MinimizeForModel(tools.ToolReadInvoice, tools.EffectResult{Outcome: tools.OutcomeSucceeded, ModelFacing: invoice})
	if err != nil {
		return security.ToolResultInput{}, Workload{}, err
	}
	input := security.ToolResultInput{
		RunID: benchmarkRunID, Tool: tools.ToolReadInvoice, ResultJSON: minimized.JSON,
		Source: security.SourceRef{SourceID: invoice.InvoiceID, Version: int64(invoice.Version)},
		Untrusted: []security.UntrustedPath{{
			Path: []string{"internal_note", "text"}, Name: security.FieldInternalNote,
			Source: security.SourceRef{SourceID: invoice.InvoiceID, Version: int64(invoice.Version), Classification: "internal_only"},
		}},
	}
	return input, Workload{
		Operation:          "policy lookup and hybrid tool-result inspection of one permitted read_invoice result",
		Tool:               tools.ToolReadInvoice,
		Fixture:            "synthetic Atlas invoice INV104 with a benign internal note",
		PayloadBytes:       len(minimized.JSON),
		UntrustedNoteBytes: len(note),
	}, nil
}

// fixtureCaller answers every security request at once with a valid low-risk verdict. It is a
// labelled fixture (verdict source "fixture"), never a model.
type fixtureCaller struct{}

func (fixtureCaller) Call(context.Context, string, string, model.Request) (model.AccountedResult, error) {
	return model.AccountedResult{Provider: model.Result{Message: model.Message{Role: "assistant", Content: fixtureVerdict}}}, nil
}

// newLiveInspector connects the semantic evaluator to the live local model in MODEL_NAME, which
// the active catalog must allow.
func newLiveInspector(catalog config.AccountingCatalog) (*security.Inspector, string, error) {
	modelSettings, err := config.LoadModel()
	if err != nil {
		return nil, "", errors.New("live model configuration unavailable; set MODEL_NAME")
	}
	if !slices.Contains(catalog.Settings.AllowedModels, modelSettings.Name) {
		return nil, "", errors.New("MODEL_NAME is not an allowed model of the active catalog")
	}
	provider, err := model.NewOllama(model.Options{BaseURL: modelSettings.BaseURL, Model: modelSettings.Name,
		Timeout: catalog.Settings.RequestTimeout, MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		return nil, "", errors.New("live model configuration invalid")
	}
	caller, err := model.NewAccountedCaller(provider, &memoryLedger{}, model.AccountingSettings{
		AgentOutputTokens: catalog.Settings.AgentOutputTokens, SecurityOutputTokens: catalog.Settings.SecurityOutputTokens,
		TemplateTokens: catalog.Settings.TemplateTokens})
	if err != nil {
		return nil, "", err
	}
	evaluator, err := security.NewSemanticEvaluator(caller, security.EvaluatorOptions{
		Model: modelSettings.Name, ContextTokens: security.MinEvaluatorContextTokens, Source: security.VerdictLive})
	if err != nil {
		return nil, "", err
	}
	return security.NewInspector(evaluator), modelSettings.Name, nil
}

// memoryLedger is an unlimited in-memory token ledger for the live run: the benchmark measures
// latency and charges no run's allowance.
type memoryLedger struct {
	mutex    sync.Mutex
	reserved int64
}

func (ledger *memoryLedger) Reserve(_ context.Context, _, _, _ string, tokens int64) (budget.Reservation, error) {
	ledger.mutex.Lock()
	defer ledger.mutex.Unlock()
	ledger.reserved += tokens
	return budget.Reservation{Tokens: tokens}, nil
}

func (ledger *memoryLedger) MarkUnknown(context.Context, string, string) error { return nil }

func (ledger *memoryLedger) Settle(_ context.Context, _, _ string, input, output int64) (budget.Settlement, error) {
	return budget.Settlement{ActualTokens: input + output}, nil
}

func (ledger *memoryLedger) Snapshot(context.Context, string) (budget.Snapshot, error) {
	ledger.mutex.Lock()
	defer ledger.mutex.Unlock()
	return budget.Snapshot{Limit: 1 << 40, Reserved: ledger.reserved}, nil
}

// readRecordedTimings aggregates runtime.timing_records per phase, without any identifier.
func readRecordedTimings(ctx context.Context, pool *pgxpool.Pool) (RecordedTimings, error) {
	recorded := RecordedTimings{
		Source: "runtime.timing_records, written by the gateway during real runs (GO-80); empty until runs have recorded spans",
		Phases: []PhaseRecord{},
	}
	readContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := pool.Query(readContext, `SELECT phase, count(*) FILTER (WHERE failed), array_agg(duration_microseconds)
		FROM runtime.timing_records GROUP BY phase ORDER BY phase`)
	if err != nil {
		return RecordedTimings{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var record PhaseRecord
		var spans []int64
		if err := rows.Scan(&record.Phase, &record.Failed, &spans); err != nil {
			return RecordedTimings{}, err
		}
		durations := make([]time.Duration, len(spans))
		for index, span := range spans {
			durations[index] = time.Duration(span) * time.Microsecond
		}
		record.Spans = summarize(durations)
		recorded.Phases = append(recorded.Phases, record)
	}
	return recorded, rows.Err()
}

func describeEnvironment(service config.Config, settings security.Settings) Environment {
	environment := Environment{
		StartedAt: time.Now().UTC(), GitCommit: "unknown", GoVersion: runtime.Version(),
		OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, CPUCount: runtime.NumCPU(), CPUModel: cpuModel(), LoadAverage: loadAverage(),
		Database:                net.JoinHostPort(service.Postgres.Host, strconv.Itoa(service.Postgres.Port)) + "/" + service.Postgres.Database,
		ActiveCatalogRevisionID: settings.EvaluatedCatalogRevisionID, Concurrency: 1,
	}
	if settings.Feed != nil {
		revision := settings.Feed.Revision
		environment.FeedRevision = &revision
	}
	if commit, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		environment.GitCommit = strings.TrimSpace(string(commit))
		if status, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
			dirty := len(strings.TrimSpace(string(status))) > 0
			environment.GitDirty = &dirty
		}
	}
	return environment
}

// cpuModel is a best-effort processor name; "unknown" when the system does not say.
func cpuModel() string {
	switch runtime.GOOS {
	case "darwin":
		if name, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			return strings.TrimSpace(string(name))
		}
	case "linux":
		if content, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for line := range strings.SplitSeq(string(content), "\n") {
				if name, found := strings.CutPrefix(line, "model name"); found {
					return strings.TrimSpace(strings.TrimLeft(name, " \t:"))
				}
			}
		}
	}
	return "unknown"
}

// loadAverage is the best-effort system load; "unknown" when the system does not say.
func loadAverage() string {
	switch runtime.GOOS {
	case "darwin":
		if load, err := exec.Command("sysctl", "-n", "vm.loadavg").Output(); err == nil {
			return strings.Trim(strings.TrimSpace(string(load)), "{} ")
		}
	case "linux":
		if content, err := os.ReadFile("/proc/loadavg"); err == nil {
			if fields := strings.Fields(string(content)); len(fields) >= 3 {
				return strings.Join(fields[:3], " ")
			}
		}
	}
	return "unknown"
}

func writeReport(report Report, outputPath string, output io.Writer) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if outputPath != "" {
		if err := os.WriteFile(outputPath, append(encoded, '\n'), 0o644); err != nil {
			return errors.New("cannot write the report file")
		}
	}
	printTable(report, output)
	_, err = fmt.Fprintf(output, "\n%s\n", encoded)
	return err
}

// printTable prints the headline numbers; the JSON below it is the full report.
func printTable(report Report, output io.Writer) {
	_, _ = fmt.Fprintf(output, "Task Passport benchmark: %s/%s, %d CPUs (%s), load %s, %s, catalog revision %d, commit %s\n",
		report.Environment.OperatingSystem, report.Environment.Architecture, report.Environment.CPUCount, report.Environment.CPUModel,
		report.Environment.LoadAverage, report.Environment.GoVersion, report.Environment.ActiveCatalogRevisionID, report.Environment.GitCommit)
	_, _ = fmt.Fprintf(output, "%-20s %8s %6s %12s %12s %12s %12s %10s\n", "configuration", "samples", "errors", "total p50 µs", "total p95 µs", "semantic p50", "provider p50", "ops/s")
	for _, configuration := range report.Configurations {
		throughput := "null"
		if configuration.ThroughputPerSecond != nil {
			throughput = strconv.FormatFloat(*configuration.ThroughputPerSecond, 'f', 1, 64)
		}
		total := configuration.Phases[phaseTotal]
		_, _ = fmt.Fprintf(output, "%-20s %8d %6d %12s %12s %12s %12s %10s\n", configuration.Name, configuration.Samples, configuration.Errors,
			text(total.Median), text(total.P95), text(configuration.Phases[phaseSemantic].Median), text(configuration.Phases[phaseProvider].Median), throughput)
	}
}

func text(value *int64) string {
	if value == nil {
		return "null"
	}
	return strconv.FormatInt(*value, 10)
}
