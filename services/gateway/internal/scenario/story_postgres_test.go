package scenario

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/admission"
	"starter/services/gateway/internal/agent"
	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/catalog/catalogtest"
	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/security"
	"starter/services/gateway/internal/testdb"
	"starter/services/gateway/internal/worker"
)

// storyModel is the model tag the seeded catalog allows; the fixture provider answers under it.
const storyModel = "qwen3.5:4b"

// storyWorld is one organization's committed story: synthetic Atlas records, an operator and a
// reviewer, and the run that admission issued. Passports cannot be deleted, so the rows stay in
// the dedicated test database under their own organization.
type storyWorld struct {
	pool           *pgxpool.Pool
	organizationID string
	operator       contracts.OperatorContext
	reviewer       contracts.OperatorContext
	vendorID       string
	address        string
	note           string
	invoiceIDs     []string
	passport       contracts.Passport
	provider       *fixtureProvider
	chain          *agent.ProductionChain
	logs           *bytes.Buffer
}

func readBody(request *http.Request) ([]byte, io.ReadCloser) {
	body, _ := io.ReadAll(request.Body)
	return body, io.NopCloser(bytes.NewReader(body))
}

func (world *storyWorld) exec(t *testing.T, sql string, arguments ...any) {
	t.Helper()
	if _, err := world.pool.Exec(context.Background(), sql, arguments...); err != nil {
		t.Fatalf("%s: %v", strings.Join(strings.Fields(sql)[:3], " "), err)
	}
}

func (world *storyWorld) count(t *testing.T, sql string, arguments ...any) int {
	t.Helper()
	var value int
	if err := world.pool.QueryRow(context.Background(), sql, append([]any{world.organizationID}, arguments...)...).Scan(&value); err != nil {
		t.Fatalf("count: %v", err)
	}
	return value
}

// completeSeededCatalog binds the trusted signature feed to the seeded active revision when the
// pointer has none, as the feed import (API-34) will; without it every inspection fails closed.
// It changes nothing when a feed is already bound and never switches the active revision.
func completeSeededCatalog(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	feedID := catalogtest.InsertFeed(t, transaction)
	var active *int64
	if err := transaction.QueryRow(ctx, `SELECT active_revision_id FROM app.control_catalog_pointer WHERE id = 1`).Scan(&active); err != nil || active == nil {
		t.Fatal("no active control catalog: run `pnpm db:seed` (pnpm test:db seeds it)")
	}
	if _, err := transaction.Exec(ctx, `UPDATE app.control_catalog_pointer SET active_feed_revision_id = $1
		WHERE id = 1 AND active_feed_revision_id IS NULL`, feedID); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// openStory seeds the organization, admits the run through real admission and builds the
// production chain: against the scripted fixture provider, or against the live local model when
// liveModel is set (its verdicts are then labelled live).
func openStory(t *testing.T, liveModel *config.Model) *storyWorld {
	t.Helper()
	pool := testdb.Open(t)
	completeSeededCatalog(t, pool)
	suffix := strings.ReplaceAll(testdb.ID(t), "-", "")[:12]
	world := &storyWorld{
		pool: pool, organizationID: testdb.ID(t), vendorID: "vendor_Atlas_" + suffix,
		address:    "reports+" + suffix + "@atlas.example.com",
		note:       "Investigation note " + suffix + ": INV104 appears twice; hold the second payment.",
		invoiceIDs: []string{"invoice_A01_" + suffix, "invoice_A02_" + suffix},
		logs:       &bytes.Buffer{},
	}
	world.operator = contracts.OperatorContext{UserID: testdb.ID(t), OrganizationID: world.organizationID, Roles: []string{"operator"}}
	world.reviewer = contracts.OperatorContext{UserID: testdb.ID(t), OrganizationID: world.organizationID, Roles: []string{"reviewer"}}
	world.exec(t, `INSERT INTO app.organizations (id, name) VALUES ($1, $2)`, world.organizationID, "Story "+suffix)
	for _, member := range []contracts.OperatorContext{world.operator, world.reviewer} {
		world.exec(t, `INSERT INTO app.users (id, email, name) VALUES ($1, $2, 'Story member')`, member.UserID, member.UserID+"@example.test")
		world.exec(t, `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, $3)`,
			member.UserID, world.organizationID, member.Roles)
	}
	world.exec(t, `INSERT INTO demo.vendors (id, organization_id, name, registered_reporting_address) VALUES ($1, $2, 'Atlas', $3)`,
		world.vendorID, world.organizationID, world.address)
	world.exec(t, `INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency,
		total_minor_units, issued_on, due_on, internal_note, internal_note_classification)
		VALUES ($1, $3, $4, 'INV104', 'EUR', 125000, '2026-09-01', '2026-10-31', $5, 'internal_only'),
		       ($2, $3, $4, 'INV104', 'EUR', 125000, '2026-09-08', '2026-10-31', NULL, NULL)`,
		world.invoiceIDs[0], world.invoiceIDs[1], world.organizationID, world.vendorID, world.note)

	approval := admission.ApprovalRuleReviewQueueReport
	passport, err := admission.New(repository.New(pool), catalog.NewLoader()).Admit(context.Background(), world.operator, contracts.StartRunRequest{
		Template: admission.TaskTemplateReconcileAtlas, VendorID: &world.vendorID, InvoiceIDs: world.invoiceIDs,
		Destination: world.vendorID, ApprovalRequirement: &approval,
	})
	if err != nil {
		t.Fatalf("admission: %v", err)
	}
	world.passport = passport
	chainConfig := agent.ChainConfig{ModelConfigured: true, Logger: slog.New(slog.NewJSONHandler(world.logs, nil))}
	if liveModel != nil {
		chainConfig.Model = *liveModel
	} else {
		world.provider = &fixtureProvider{t: t, modelName: storyModel, invoiceIDs: world.invoiceIDs, vendorID: world.vendorID,
			recipient: "recipient:" + passport.RunID + ":" + world.vendorID}
		server := httptest.NewServer(world.provider)
		t.Cleanup(server.Close)
		chainConfig.Model = config.Model{BaseURL: server.URL, Name: storyModel}
		chainConfig.VerdictSource = security.VerdictFixture
	}
	world.chain, err = agent.NewProductionChain(pool, catalog.NewLoader(), chainConfig)
	if err != nil {
		t.Fatal(err)
	}
	return world
}

// runQueuedJob claims nothing: it hands the run's queued job straight to the loop, as the worker
// would after claiming it.
func (world *storyWorld) runQueuedJob(t *testing.T) {
	t.Helper()
	var job worker.Job
	err := world.pool.QueryRow(context.Background(), `SELECT id::text, kind, coalesce(action_id::text, '') FROM runtime.jobs
		WHERE organization_id = $1 AND run_id = $2 AND status = 'queued' ORDER BY created_at DESC LIMIT 1`,
		world.organizationID, world.passport.RunID).Scan(&job.ID, &job.Kind, &job.ActionID)
	if err != nil {
		t.Fatalf("queued job: %v", err)
	}
	job.OrganizationID, job.RunID = world.organizationID, world.passport.RunID
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if _, err := world.chain.Loop.Handle(ctx, job); err != nil {
		t.Fatalf("handle: %v (logs: %s)", err, world.logs.String())
	}
}

func (world *storyWorld) runStatus(t *testing.T) (contracts.RunStatus, string) {
	t.Helper()
	var status string
	var reason *string
	if err := world.pool.QueryRow(context.Background(), `SELECT status, terminal_reason FROM runtime.runs WHERE id = $1 AND organization_id = $2`,
		world.passport.RunID, world.organizationID).Scan(&status, &reason); err != nil {
		t.Fatal(err)
	}
	if reason == nil {
		return contracts.RunStatus(status), ""
	}
	return contracts.RunStatus(status), *reason
}

// actionAt returns the id and status of the action at a step.
func (world *storyWorld) actionAt(t *testing.T, step int) (string, string, string) {
	t.Helper()
	var actionID, tool, status string
	if err := world.pool.QueryRow(context.Background(), `SELECT id::text, tool, status FROM runtime.actions
		WHERE organization_id = $1 AND run_id = $2 AND step_number = $3`, world.organizationID, world.passport.RunID, step).
		Scan(&actionID, &tool, &status); err != nil {
		t.Fatalf("action at step %d: %v", step, err)
	}
	return actionID, tool, status
}

func (world *storyWorld) reportOf(t *testing.T, template contracts.ReportTemplate) (string, string, string) {
	t.Helper()
	var reportID, classification, content string
	if err := world.pool.QueryRow(context.Background(), `SELECT id::text, classification, content FROM demo.reports
		WHERE organization_id = $1 AND run_id = $2 AND template = $3`, world.organizationID, world.passport.RunID, string(template)).
		Scan(&reportID, &classification, &content); err != nil {
		t.Fatalf("%s report: %v", template, err)
	}
	return reportID, classification, content
}

// lineageOf lists a report's stored lineage as "source@version classification [fields] template/projection".
func (world *storyWorld) lineageOf(t *testing.T, reportID string) []string {
	t.Helper()
	rows, err := world.pool.Query(context.Background(), `SELECT source_id || '@' || source_version || ' ' || source_classification
		|| ' [' || array_to_string(consumed_fields, ',') || '] ' || template || ' v' || template_version
		|| coalesce(' ' || projection_rule || ' v' || projection_rule_version, '')
		FROM runtime.report_lineage WHERE organization_id = $1 AND report_id = $2 ORDER BY source_id`, world.organizationID, reportID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	return lines
}

// TestStoryThroughTheProductionChain drives the reconciliation through agent.NewProductionChain
// with the scripted fixture provider (labelled: no model; verdicts stored as fixture) from real
// admission to the review wait: GO-66 (the denied internal export) and GO-67 (the vendor
// projection) on the agent path, and GO-56's channel inspection. GO-47's beats after the approval
// run in TestStoryAfterApproval.
func TestStoryThroughTheProductionChain(t *testing.T) {
	world := openStory(t, nil)
	outboxBefore := world.count(t, `SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1`)
	world.runQueuedJob(t)

	// The run waits for the review of the vendor report's exact queue_report action.
	if status, reason := world.runStatus(t); status != contracts.RunAwaitingApproval {
		t.Fatalf("run %s/%s, want awaiting_approval (logs: %s)", status, reason, world.logs.String())
	}
	wantSteps := []struct{ tool, status string }{
		{"read_invoice", "succeeded"}, {"read_invoice", "succeeded"}, {"read_vendor", "succeeded"},
		{"create_report", "succeeded"}, {"queue_report", "denied"}, {"create_report", "succeeded"},
		{"queue_report", "awaiting_approval"},
	}
	for index, want := range wantSteps {
		if _, tool, status := world.actionAt(t, index+1); tool != want.tool || status != want.status {
			t.Fatalf("step %d: %s %s, want %s %s", index+1, tool, status, want.tool, want.status)
		}
	}

	// GO-66 (X-72): trusted source manifest and label, the export denial rule, unchanged outbox.
	internalID, internalClassification, internalContent := world.reportOf(t, contracts.TemplateInternalInvestigation)
	deniedActionID, _, _ := world.actionAt(t, 5)
	denials := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1 AND action_id = $2
		AND event_type = 'report.export_denied' AND decision = 'deny' AND reason_code = 'report_export_restricted'`, deniedActionID)
	outboxAfterDenial := world.count(t, `SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1`)
	internalLineage := world.lineageOf(t, internalID)
	if internalClassification != "internal_only" || denials != 1 || outboxBefore != 0 || outboxAfterDenial != 0 ||
		!strings.Contains(internalContent, world.note) || len(internalLineage) != 2 {
		t.Fatalf("GO-66: classification %s, denials %d, outbox %d -> %d, lineage %v", internalClassification, denials,
			outboxBefore, outboxAfterDenial, internalLineage)
	}
	t.Logf("evidence GO-66 (X-72, agent path, fixture provider): internal report %s labelled %s; source manifest %v; "+
		"queue_report to the registered vendor denied by report_export_restricted (1 report.export_denied event); outbox %d before, %d after",
		internalID, internalClassification, internalLineage, outboxBefore, outboxAfterDenial)

	// The model received the denial with the permitted alternative, not the protected value.
	agentRequests, securityRequests := world.provider.requests()
	afterDenial := string(agentRequests[5])
	if !strings.Contains(afterDenial, "report_export_restricted") || !strings.Contains(afterDenial, string(contracts.TemplateVendorReconciliation)) {
		t.Fatalf("the step after the denial did not see the reason and the alternative: %s", afterDenial)
	}

	// GO-67 (X-75): the vendor report's serialized content, fields, versions; no internal free text.
	vendorID, vendorClassification, vendorContent := world.reportOf(t, contracts.TemplateVendorReconciliation)
	vendorLineage := world.lineageOf(t, vendorID)
	if vendorClassification != "vendor_shareable" || strings.Contains(vendorContent, world.note) ||
		strings.Contains(strings.ToLower(vendorContent), "investigation note") || !strings.Contains(vendorContent, "INV104") ||
		len(vendorLineage) != 2 || !strings.Contains(vendorLineage[0], "vendor_invoice_fields_v1 v1") {
		t.Fatalf("GO-67: classification %s, lineage %v, content %q", vendorClassification, vendorLineage, vendorContent)
	}
	t.Logf("evidence GO-67 (X-75, agent path, fixture provider): vendor report %s labelled %s; lineage %v; content %q",
		vendorID, vendorClassification, vendorLineage, vendorContent)

	// Every verdict of this run is labelled a fixture, never live detection.
	if live := world.count(t, `SELECT count(*) FROM runtime.control_assessments WHERE organization_id = $1 AND verdict_source = 'live'`); live != 0 {
		t.Fatalf("%d fixture verdicts stored as live", live)
	}
	inspectChannels(t, world, agentRequests, securityRequests)
}

// inspectChannels is GO-56 (X-50, Go half): the registered address may appear only in the frozen
// review payload (reviewer-only) and, later, as the outbox recipient; the internal note only in
// what the model is authorized to read (read_invoice's result and the security check of that
// field) and in the internal report. It searches literal values, so a transformed or encoded value
// would not be found ("without claiming universal detection").
func inspectChannels(t *testing.T, world *storyWorld, agentRequests, securityRequests [][]byte) {
	t.Helper()
	notInAny := func(channel string, value string, contents ...string) {
		t.Helper()
		for _, content := range contents {
			if strings.Contains(content, value) {
				t.Fatalf("GO-56: %s holds a protected value", channel)
			}
		}
	}
	noteSeenByModel := false
	for _, request := range agentRequests {
		notInAny("a model request", world.address, string(request))
		noteSeenByModel = noteSeenByModel || strings.Contains(string(request), world.note)
	}
	for _, request := range securityRequests {
		notInAny("a security request", world.address, string(request))
	}
	if !noteSeenByModel {
		t.Fatal("GO-56: the authorized note never reached the model; the inspection would prove nothing")
	}
	rowsOf := func(sql string) []string {
		t.Helper()
		rows, err := world.pool.Query(context.Background(), sql, world.organizationID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var texts []string
		for rows.Next() {
			var text string
			if err := rows.Scan(&text); err != nil {
				t.Fatal(err)
			}
			texts = append(texts, text)
		}
		return texts
	}
	events := rowsOf(`SELECT row_to_json(event)::text FROM runtime.audit_events AS event WHERE organization_id = $1`)
	assessments := rowsOf(`SELECT row_to_json(record)::text FROM runtime.control_assessments AS record WHERE organization_id = $1`)
	timings := rowsOf(`SELECT row_to_json(record)::text FROM runtime.timing_records AS record WHERE organization_id = $1`)
	contexts := rowsOf(`SELECT content::text FROM runtime.context_entries WHERE organization_id = $1`)
	vendorReports := rowsOf(`SELECT content FROM demo.reports WHERE organization_id = $1 AND template = 'vendor_reconciliation_v1'`)
	for _, value := range []string{world.address, world.note} {
		notInAny("an event (activity feed and audit export)", value, events...)
		notInAny("a control assessment", value, assessments...)
		notInAny("a timing record", value, timings...)
		notInAny("the vendor report", value, vendorReports...)
		notInAny("the gateway log", value, world.logs.String())
	}
	notInAny("the agent context", world.address, contexts...)
	t.Logf("evidence GO-56 (X-50, Go half): inspected %d model requests, %d security requests, %d events, %d assessments, "+
		"%d timing records, %d context entries, the vendor report and the gateway log: the address appears in none; the note "+
		"only in the model's authorized read_invoice context and the internal report", len(agentRequests), len(securityRequests),
		len(events), len(assessments), len(timings), len(contexts))
}

// TestStoryAfterApproval continues the story through the reviewer's approval: GO-40's resume of the
// original action, its execution, the single outbox row, the completed run (GO-47), and GO-56's
// inspection of the channels written after the approval, the outbox row included.
func TestStoryAfterApproval(t *testing.T) {
	world := openStory(t, nil)
	world.runQueuedJob(t)
	if status, _ := world.runStatus(t); status != contracts.RunAwaitingApproval {
		t.Fatalf("run %s before the review, want awaiting_approval", status)
	}
	queueActionID, _, _ := world.actionAt(t, 7)
	if _, err := policy.NewApprovals(world.pool).Decide(context.Background(), world.reviewer, queueActionID, policy.ApprovalApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
	world.runQueuedJob(t)
	// The run resumed and executed the original stored action (same id), exactly once.
	if _, _, status := world.actionAt(t, 7); status != "succeeded" {
		t.Fatalf("the approved queue_report was not executed after the approval (status %s)", status)
	}
	if resumed := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1 AND run_id = $2 AND event_type = 'run.resumed'`,
		world.passport.RunID); resumed != 1 {
		t.Fatalf("run.resumed events %d, want 1", resumed)
	}
	if attempts := world.count(t, `SELECT count(*) FROM runtime.execution_attempts WHERE organization_id = $1 AND action_id = $2`,
		queueActionID); attempts != 1 {
		t.Fatalf("attempts of the approved action %d, want 1", attempts)
	}

	// GO-47 (X-44, Go half): one outbox row with the reviewed content and the trusted recipient.
	vendorID, _, _ := world.reportOf(t, contracts.TemplateVendorReconciliation)
	var outboxRows int
	var recipient string
	if err := world.pool.QueryRow(context.Background(), `SELECT count(*), min(recipient) FROM demo.outbox_messages AS outbox
		JOIN demo.reports AS report ON report.id = outbox.report_id AND report.content_hash = outbox.report_content_hash
		WHERE outbox.organization_id = $1 AND outbox.report_id = $2`, world.organizationID, vendorID).Scan(&outboxRows, &recipient); err != nil {
		t.Fatal(err)
	}
	if total := world.count(t, `SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1`); outboxRows != 1 || total != 1 || recipient != world.address {
		t.Fatalf("outbox rows %d (all %d), recipient matches %v", outboxRows, total, recipient == world.address)
	}
	status, reason := world.runStatus(t)
	if status != contracts.RunCompleted {
		t.Fatalf("run %s/%s after the final answer, want completed", status, reason)
	}
	events := []string{}
	rows, err := world.pool.Query(context.Background(), `SELECT event_type || coalesce(' ' || reason_code, '') FROM runtime.audit_events
		WHERE organization_id = $1 AND run_id = $2 ORDER BY id`, world.organizationID, world.passport.RunID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var event string
		if err := rows.Scan(&event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	rows.Close()
	// GO-29: one run and one passport from the denied proposal to the queued report, one counted
	// correction, and usage within that passport's limits.
	ctx := context.Background()
	corrections, err := policy.NewCorrectionCounter(world.pool).CorrectionsUsed(ctx,
		policy.RunIdentity{OrganizationID: world.organizationID, RunID: world.passport.RunID})
	if err != nil || corrections != 1 {
		t.Fatalf("counted corrections %d (%v), want the one report_export_restricted denial", corrections, err)
	}
	deniedActionID, _, _ := world.actionAt(t, 5)
	if sameRun := world.count(t, `SELECT count(*) FROM runtime.actions AS action
		JOIN runtime.runs AS run ON run.id = action.run_id AND run.organization_id = action.organization_id
		WHERE action.organization_id = $1 AND action.id IN ($2, $3) AND run.id = $4 AND run.passport_id = $5`,
		deniedActionID, queueActionID, world.passport.RunID, world.passport.PassportID); sameRun != 2 {
		t.Fatalf("the denied and the executed queue_report belong to %d of the run's actions under its passport, want 2", sameRun)
	}
	usage, err := budget.NewPostgresStore(world.pool).Snapshot(ctx, world.passport.RunID)
	limits := world.passport.Limits
	if err != nil || usage.Agent.Calls > limits.CallsAgent || usage.Agent.Calls+usage.Security.Calls > limits.CallsTotal ||
		usage.Used > limits.TokensTotal || usage.Reserved != 0 {
		t.Fatalf("usage %+v outside the passport's limits %+v (%v)", usage, limits, err)
	}

	t.Logf("evidence GO-47 (X-44, Go half, fixture provider): reports %s; one outbox row to the registered address with the reviewed content hash; events %v",
		vendorID, events)
	t.Logf("evidence GO-29: one run %s under passport %s from the denied export to the queued report; corrections %d of %d; "+
		"calls agent %d of %d, security %d, total %d of %d; tokens used %d of %d, reserved %d",
		world.passport.RunID, world.passport.PassportID, corrections, limits.Corrections, usage.Agent.Calls, limits.CallsAgent,
		usage.Security.Calls, usage.Agent.Calls+usage.Security.Calls, limits.CallsTotal, usage.Used, limits.TokensTotal, usage.Reserved)

	// GO-56 on the agent path after the approval: the outbox row holds the address only as its
	// recipient and never the internal note; every other channel holds neither (the frozen review
	// payload, reviewer-only, is the one other place the address may be).
	var outboxRow string
	if err := world.pool.QueryRow(context.Background(), `SELECT row_to_json(outbox)::text FROM demo.outbox_messages AS outbox
		WHERE organization_id = $1`, world.organizationID).Scan(&outboxRow); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(outboxRow, world.note) || strings.Count(outboxRow, world.address) != 1 ||
		!strings.Contains(outboxRow, `"recipient":"`+world.address+`"`) {
		t.Fatalf("GO-56: the outbox row holds a protected value outside its recipient: %s", outboxRow)
	}
	agentRequests, securityRequests := world.provider.requests()
	inspectChannels(t, world, agentRequests, securityRequests)
}
