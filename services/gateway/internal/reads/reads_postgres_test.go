package reads

import (
	"context"
	"crypto/sha256"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/testdb"
)

// execer is a pool or a transaction.
type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func insert(t *testing.T, database execer, sql string, arguments ...any) {
	t.Helper()
	if _, err := database.Exec(context.Background(), sql, arguments...); err != nil {
		t.Fatalf("%s: %v", strings.Join(strings.Fields(sql)[:3], " "), err)
	}
}

// openTransaction returns a transaction that the test rolls back: passports cannot be deleted.
func openTransaction(t *testing.T) (*pgxpool.Pool, pgx.Tx) {
	t.Helper()
	pool := testdb.Open(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return pool, tx
}

// newRun issues a passport and a running run of the organization and returns the run id.
func newRun(t *testing.T, database execer, organizationID string) string {
	t.Helper()
	passportID, runID := testdb.ID(t), testdb.ID(t)
	insert(t, database, `INSERT INTO runtime.passports (id, organization_id, actor_id, task_version,
	                 admission_catalog_revision_id, scope, limits, expires_at)
	               VALUES ($1, $2, $3, 'reconcile_atlas_v1', 1, '{}', '{}', now() + interval '15 minutes')`,
		passportID, organizationID, testdb.ID(t))
	insert(t, database, `INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
		runID, organizationID, passportID)
	return runID
}

// newAction stores an action of the run and returns its id.
func newAction(t *testing.T, database execer, organizationID, runID string, step int) string {
	t.Helper()
	actionID := testdb.ID(t)
	digest := sha256.Sum256([]byte(actionID))
	insert(t, database, `INSERT INTO runtime.actions (id, organization_id, run_id, step_number, tool, canonical_arguments,
	                 canonicalization_version, action_digest, idempotency_key, evaluated_catalog_revision_id, status)
	               VALUES ($1, $2, $3, $4, 'read_invoice', '{}', 1, $5, $6, 1, 'executing')`,
		actionID, organizationID, runID, step, digest[:], testdb.ID(t))
	return actionID
}

func operatorOf(organizationID string) *contracts.OperatorContext {
	return &contracts.OperatorContext{UserID: testOperator.UserID, OrganizationID: organizationID, Roles: []string{"operator"}}
}

// eventually polls until done returns true. Organization-wide pages only return rows whose
// transactions are final, and parallel test packages hold short transactions of their own.
func eventually(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestPostgresRunUsageCountsRecordedUsageOnly(t *testing.T) {
	_, tx := openTransaction(t)
	organizationID := testdb.ID(t)
	runID := newRun(t, tx, organizationID)
	ledgerlessRunID := newRun(t, tx, organizationID)

	callIDs := make([]string, 4)
	for index, call := range []struct{ purpose, outcome string }{
		{"agent", "completed"}, {"agent", "completed"}, {"agent", "usage_unknown"}, {"security", ""},
	} {
		var outcome any
		if call.outcome != "" {
			outcome = call.outcome
		}
		callIDs[index] = testdb.ID(t)
		insert(t, tx, `INSERT INTO runtime.model_calls (id, organization_id, run_id, purpose, model, outcome)
		               VALUES ($1, $2, $3, $4, 'local-model', $5)`, callIDs[index], organizationID, runID, call.purpose, outcome)
	}
	insert(t, tx, `INSERT INTO runtime.model_token_budgets (run_id, organization_id, token_limit, reserved_tokens, used_tokens,
	                 call_limit, agent_call_limit, security_call_limit, request_timeout_ms, max_concurrent_calls)
	               VALUES ($1, $2, 4000, 50, 300, 24, 12, 12, 20000, 2)`, runID, organizationID)
	insert(t, tx, `INSERT INTO runtime.model_token_reservations (run_id, organization_id, call_id, purpose, token_reservation, status, input_tokens, output_tokens, actual_tokens)
	               VALUES ($1, $6, $2, 'agent', 400, 'settled', 100, 50, 150), ($1, $6, $3, 'agent', 400, 'settled', 120, 30, 150),
	                      ($1, $6, $4, 'agent', 40, 'usage_unknown', NULL, NULL, NULL), ($1, $6, $5, 'security', 10, 'reserved', NULL, NULL, NULL)`,
		runID, callIDs[0], callIDs[1], callIDs[2], callIDs[3], organizationID)
	firstAction := newAction(t, tx, organizationID, runID, 1)
	secondAction := newAction(t, tx, organizationID, runID, 2)
	insert(t, tx, `INSERT INTO runtime.execution_attempts (organization_id, action_id, attempt_number, outcome, completed_at)
	               VALUES ($1, $2, 1, 'failed', now()), ($1, $2, 2, 'succeeded', now()), ($1, $3, 1, 'aborted', now())`,
		organizationID, firstAction, secondAction)
	insert(t, tx, `INSERT INTO runtime.execution_attempts (organization_id, action_id, attempt_number) VALUES ($1, $2, 2)`,
		organizationID, secondAction)

	recorder := serve(t, RunUsageRoutePattern, RunUsageHandler(tx), "/internal/runs/"+runID+"/usage", operatorOf(organizationID))
	var usage RunUsage
	if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &usage) != nil {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	want := RunUsage{
		RunID: runID,
		ModelCalls: []PurposeUsage{
			{Purpose: "agent", Dispatched: 3, Completed: 2, UsageUnknown: 1, SettledTokens: 300, HeldTokens: 40, UsageUnknownReservations: 1},
			{Purpose: "security", Dispatched: 1, InFlight: 1, HeldTokens: 10},
		},
		Tokens:       &TokenLedger{Limit: 4000, Reserved: 50, Used: 300},
		ToolAttempts: ToolAttemptUsage{Total: 4, Succeeded: 1, Failed: 1, Aborted: 1, Open: 1},
	}
	if !slices.Equal(usage.ModelCalls, want.ModelCalls) || *usage.Tokens != *want.Tokens || usage.ToolAttempts != want.ToolAttempts || usage.RunID != runID {
		t.Fatalf("usage %+v\nwant  %+v", usage, want)
	}
	t.Logf("evidence GO-24: usage view %s", recorder.Body.String())

	// A run without a ledger has null tokens and zero counts, never made-up values.
	recorder = serve(t, RunUsageRoutePattern, RunUsageHandler(tx), "/internal/runs/"+ledgerlessRunID+"/usage", operatorOf(organizationID))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"tokens":null`) {
		t.Fatalf("ledgerless run: %d %s", recorder.Code, recorder.Body.String())
	}

	// Another organization's operator learns nothing about the run.
	recorder = serve(t, RunUsageRoutePattern, RunUsageHandler(tx), "/internal/runs/"+runID+"/usage", operatorOf(testdb.ID(t)))
	expectError(t, recorder, http.StatusNotFound, "not_found")
	if strings.Contains(recorder.Body.String(), "agent") {
		t.Fatal("usage in the rejection body")
	}

	// An attempt outcome this view does not know is not folded into another count.
	insert(t, tx, `INSERT INTO runtime.execution_attempts (organization_id, action_id, attempt_number, outcome, completed_at)
	               VALUES ($1, $2, 3, 'vanished', now())`, organizationID, secondAction)
	expectError(t, serve(t, RunUsageRoutePattern, RunUsageHandler(tx), "/internal/runs/"+runID+"/usage", operatorOf(organizationID)),
		http.StatusServiceUnavailable, "unavailable")
}

func TestPostgresRunStateAndEventsAreServedThroughTheRepository(t *testing.T) {
	_, tx := openTransaction(t)
	organizationID := testdb.ID(t)
	runID := newRun(t, tx, organizationID)
	eventTx := repository.Join(tx)
	var appended []string
	for _, eventType := range []contracts.EventType{contracts.EventRunStarted, contracts.EventActionProposed, contracts.EventActionAllowed} {
		event, err := eventTx.AppendEvent(context.Background(), repository.NewEvent{OrganizationID: organizationID, RunID: &runID, EventType: eventType})
		if err != nil {
			t.Fatal(err)
		}
		appended = append(appended, event.EventID)
	}
	runs := repository.New(tx)

	recorder := serve(t, RunStateRoutePattern, RunStateHandler(runs), "/internal/runs/"+runID, operatorOf(organizationID))
	var state contracts.RunState
	if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &state) != nil || state.RunID != runID || state.Status != contracts.RunRunning {
		t.Fatalf("state %d: %s", recorder.Code, recorder.Body.String())
	}

	var seen []string
	cursor := "0"
	for range 3 {
		recorder = serve(t, RunEventsRoutePattern, RunEventsHandler(runs), "/internal/runs/"+runID+"/events?limit=2&after="+cursor, operatorOf(organizationID))
		var page RunEventPage
		if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &page) != nil {
			t.Fatalf("events %d: %s", recorder.Code, recorder.Body.String())
		}
		for _, event := range page.Events {
			seen = append(seen, event.EventID)
		}
		cursor = page.NextCursor
	}
	if !slices.Equal(seen, appended) {
		t.Fatalf("paged %v, appended %v", seen, appended)
	}

	other := operatorOf(testdb.ID(t))
	expectError(t, serve(t, RunStateRoutePattern, RunStateHandler(runs), "/internal/runs/"+runID, other), http.StatusNotFound, "not_found")
	expectError(t, serve(t, RunEventsRoutePattern, RunEventsHandler(runs), "/internal/runs/"+runID+"/events", other), http.StatusNotFound, "not_found")
}

// readEvents pages the organization's events from cursor until a page comes back empty and
// returns the ids and the cursor to continue from.
func readEvents(t *testing.T, database Beginner, organizationID, cursor string) ([]string, string) {
	t.Helper()
	var ids []string
	for {
		target := "/internal/security/events?limit=2"
		if cursor != "" {
			target += "&cursor=" + cursor
		}
		recorder := serve(t, SecurityEventsRoutePattern, SecurityEventsHandler(database), target, operatorOf(organizationID))
		var page SecurityEventPage
		if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &page) != nil {
			t.Fatalf("events %d: %s", recorder.Code, recorder.Body.String())
		}
		cursor = page.NextCursor
		if len(page.Events) == 0 {
			return ids, cursor
		}
		for _, event := range page.Events {
			if event.OrganizationID != organizationID {
				t.Fatalf("event of another organization: %+v", event)
			}
			ids = append(ids, event.EventID)
		}
	}
}

// TestPostgresOrganizationEventsAreReadExactlyOnceAcrossOpenTransactions is the case an id cursor
// gets wrong: a transaction holding a lower id commits after one holding a higher id.
func TestPostgresOrganizationEventsAreReadExactlyOnceAcrossOpenTransactions(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	organizationID, otherOrganizationID := testdb.ID(t), testdb.ID(t)
	appendRunless := func(tx pgx.Tx, organization string) string {
		t.Helper()
		event, err := repository.Join(tx).AppendEvent(ctx, repository.NewEvent{OrganizationID: organization, EventType: contracts.EventAdmissionRejected})
		if err != nil {
			t.Fatal(err)
		}
		return event.EventID
	}

	older, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = older.Rollback(ctx) }()
	// The older transaction takes its transaction id first, but its event id comes later.
	if _, err := older.Exec(ctx, `SELECT pg_current_xact_id()`); err != nil {
		t.Fatal(err)
	}
	newer, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = newer.Rollback(ctx) }()
	lowerID := appendRunless(newer, organizationID)
	higherID := appendRunless(older, organizationID)
	if err := older.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	other, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	appendRunless(other, otherOrganizationID)
	if err := other.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// While the lower id is still open, the committed higher id is served; an id cursor would now
	// stand past the lower id and never return it.
	var seen []string
	cursor := ""
	eventually(t, "the committed event", func() bool {
		var ids []string
		ids, cursor = readEvents(t, pool, organizationID, cursor)
		seen = append(seen, ids...)
		return len(seen) > 0
	})
	if !slices.Equal(seen, []string{higherID}) {
		t.Fatalf("while open: read %v, want only %s (lower %s still open)", seen, higherID, lowerID)
	}
	if err := newer.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the late event", func() bool {
		var ids []string
		ids, cursor = readEvents(t, pool, organizationID, cursor)
		seen = append(seen, ids...)
		return len(seen) > 1
	})
	// A further read returns nothing new: every event exactly once.
	more, _ := readEvents(t, pool, organizationID, cursor)
	seen = append(seen, more...)
	if !slices.Equal(seen, []string{higherID, lowerID}) {
		t.Fatalf("read %v, want %s then %s, each once", seen, higherID, lowerID)
	}
	t.Logf("evidence GO-83: event %s committed before event %s; the window cursor read %v", higherID, lowerID, seen)
}

func TestPostgresSecurityRecordsAndSummary(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	organizationID := testdb.ID(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	runID := newRun(t, tx, organizationID)
	actionID := newAction(t, tx, organizationID, runID, 1)
	securityCallID := testdb.ID(t)
	insert(t, tx, `INSERT INTO runtime.model_calls (id, organization_id, run_id, purpose, model, outcome)
	               VALUES ($1, $2, $3, 'security', 'local-model', 'completed')`, securityCallID, organizationID, runID)
	evaluationID := testdb.ID(t)
	insert(t, tx, `INSERT INTO runtime.control_assessments (organization_id, run_id, evaluation_id, action_id, security_model_call_id,
	                 boundary, control_class, control_id, outcome, reason_code, admission_catalog_revision_id,
	                 evaluated_catalog_revision_id, matched_rule_id, feed_revision, verdict_source, verdict)
	               VALUES ($1, $2, $3, $4, NULL, 'tool_result', 'deterministic', 'signature_match', 'block', 'signature_match', 1, 1, 'sig_override_001', 'feed_v1', NULL, NULL),
	                      ($1, $2, $3, $4, $5, 'tool_result', 'semantic', 'semantic_injection', 'block', 'semantic_injection_detected', 1, 1, NULL, NULL, 'live',
	                       '{"risk_category":"prompt_injection","score":0.93,"reason_code":"semantic_injection_detected"}'),
	                      ($1, $2, $3, $4, NULL, 'tool_result', 'semantic', 'semantic_injection', 'pass', NULL, 1, 1, NULL, NULL, 'fixture',
	                       '{"risk_category":"none","score":0.02,"reason_code":"none"}')`,
		organizationID, runID, evaluationID, actionID, securityCallID)
	for index, duration := range []int{100, 200, 300, 400, 5000} {
		insert(t, tx, `INSERT INTO runtime.timing_records (organization_id, run_id, evaluation_id, phase, duration_microseconds, failed)
		               VALUES ($1, $2, $3, 'deterministic', $4, $5)`, organizationID, runID, evaluationID, duration, index == 4)
	}
	if _, err := repository.Join(tx).AppendEvent(ctx, repository.NewEvent{OrganizationID: organizationID, RunID: &runID,
		EventType: contracts.EventControlEvaluated, Decision: pointer(contracts.DecisionDeny), ReasonCode: pointer(contracts.ReasonSignatureMatch)}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var records []AssessmentRecord
	var rawPages []string
	cursor := ""
	eventually(t, "the committed assessments", func() bool {
		target := "/internal/security/assessments?limit=2"
		if cursor != "" {
			target += "&cursor=" + cursor
		}
		recorder := serve(t, SecurityAssessmentsRoutePattern, SecurityAssessmentsHandler(pool), target, operatorOf(organizationID))
		var page AssessmentPage
		if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &page) != nil {
			t.Fatalf("assessments %d: %s", recorder.Code, recorder.Body.String())
		}
		rawPages = append(rawPages, recorder.Body.String())
		records = append(records, page.Records...)
		cursor = page.NextCursor
		return len(records) >= 3
	})
	if len(records) != 3 || records[0].ControlClass != "deterministic" || records[0].VerdictSource != nil ||
		*records[0].MatchedRuleID != "sig_override_001" || *records[1].VerdictSource != "live" || records[1].Verdict.Score != 0.93 ||
		*records[1].SecurityModelCallID != securityCallID || *records[2].VerdictSource != "fixture" {
		t.Fatalf("records %+v", records)
	}
	t.Logf("evidence GO-83: assessment pages %s", strings.Join(rawPages, " | "))

	recorder := serve(t, SecuritySummaryRoutePattern, SecuritySummaryHandler(pool), "/internal/security/summary", operatorOf(organizationID))
	var summary SecuritySummary
	if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &summary) != nil {
		t.Fatalf("summary %d: %s", recorder.Code, recorder.Body.String())
	}
	live, fixture := "live", "fixture"
	wantAssessments := []AssessmentCount{
		{ControlClass: "deterministic", ControlID: "signature_match", Outcome: "block", Count: 1},
		{ControlClass: "semantic", ControlID: "semantic_injection", Outcome: "block", VerdictSource: &live, Count: 1},
		{ControlClass: "semantic", ControlID: "semantic_injection", Outcome: "pass", VerdictSource: &fixture, Count: 1},
	}
	if len(summary.Assessments) != len(wantAssessments) {
		t.Fatalf("assessment counts %+v", summary.Assessments)
	}
	for index, want := range wantAssessments {
		got := summary.Assessments[index]
		if got.ControlClass != want.ControlClass || got.ControlID != want.ControlID || got.Outcome != want.Outcome ||
			got.Count != want.Count || (got.VerdictSource == nil) != (want.VerdictSource == nil) ||
			(got.VerdictSource != nil && *got.VerdictSource != *want.VerdictSource) {
			t.Fatalf("assessment count %d: %+v, want %+v", index, got, want)
		}
	}
	wantTiming := PhaseTiming{Phase: "deterministic", Count: 5, Failed: 1, MedianMicroseconds: 300, P95Microseconds: 5000, MaxMicroseconds: 5000}
	if len(summary.Timings) != 1 || summary.Timings[0] != wantTiming ||
		len(summary.Runs) != 1 || summary.Runs[0] != (StatusCount{Status: contracts.RunRunning, Count: 1}) ||
		len(summary.Decisions) != 1 || summary.Decisions[0].Count != 1 || *summary.Decisions[0].ReasonCode != contracts.ReasonSignatureMatch ||
		summary.ModelUsage[1].Completed != 1 || summary.OrganizationID != organizationID {
		t.Fatalf("summary %s", recorder.Body.String())
	}
	t.Logf("evidence GO-83: summary %s", recorder.Body.String())

	// Another organization sees none of it.
	recorder = serve(t, SecuritySummaryRoutePattern, SecuritySummaryHandler(pool), "/internal/security/summary", operatorOf(testdb.ID(t)))
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "signature_match") || strings.Contains(recorder.Body.String(), runID) {
		t.Fatalf("other organization summary: %s", recorder.Body.String())
	}
	recorder = serve(t, SecurityAssessmentsRoutePattern, SecurityAssessmentsHandler(pool), "/internal/security/assessments", operatorOf(testdb.ID(t)))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"records":[]`) {
		t.Fatalf("other organization assessments: %s", recorder.Body.String())
	}
}

func TestPostgresAVerdictWithAnExtraKeyIsNeverServed(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	organizationID := testdb.ID(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	runID := newRun(t, tx, organizationID)
	insert(t, tx, `INSERT INTO runtime.control_assessments (organization_id, run_id, evaluation_id, boundary, control_class, control_id,
	                 outcome, admission_catalog_revision_id, evaluated_catalog_revision_id, verdict_source, verdict)
	               VALUES ($1, $2, $3, 'tool_result', 'semantic', 'semantic_injection', 'pass', 1, 1, 'live',
	                       '{"risk_category":"none","score":0.1,"reason_code":"none","reasoning":"RAW CLASSIFIER TEXT"}')`,
		organizationID, runID, testdb.ID(t))
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var last string
	eventually(t, "the refusal", func() bool {
		recorder := serve(t, SecurityAssessmentsRoutePattern, SecurityAssessmentsHandler(pool), "/internal/security/assessments", operatorOf(organizationID))
		last = recorder.Body.String()
		if strings.Contains(last, "RAW CLASSIFIER TEXT") {
			t.Fatalf("classifier text served: %s", last)
		}
		return recorder.Code == http.StatusServiceUnavailable
	})
	if !strings.Contains(last, `"unavailable"`) {
		t.Fatalf("refusal %s", last)
	}
}

func TestPostgresReadsWorkAsTheGatewayRole(t *testing.T) {
	_, tx := openTransaction(t)
	organizationID := testdb.ID(t)
	runID := newRun(t, tx, organizationID)
	if _, err := tx.Exec(context.Background(), `SET LOCAL ROLE task_passport_gateway`); err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct {
		pattern, target string
		handler         http.Handler
	}{
		{RunUsageRoutePattern, "/internal/runs/" + runID + "/usage", RunUsageHandler(tx)},
		{SecuritySummaryRoutePattern, "/internal/security/summary", SecuritySummaryHandler(tx)},
		{SecurityAssessmentsRoutePattern, "/internal/security/assessments", SecurityAssessmentsHandler(tx)},
		{SecurityEventsRoutePattern, "/internal/security/events", SecurityEventsHandler(tx)},
	} {
		recorder := serve(t, request.pattern, request.handler, request.target, operatorOf(organizationID))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s as the gateway role: %d %s", request.target, recorder.Code, recorder.Body.String())
		}
	}
}

func pointer[Value any](value Value) *Value { return &value }
