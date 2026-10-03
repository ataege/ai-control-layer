package reads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/health"
	"starter/services/gateway/internal/operatorcontext"
	"starter/services/gateway/internal/repository"
)

const fixturesDirectory = "../../../../packages/contracts/fixtures"

var testOperator = contracts.OperatorContext{
	UserID:         "0b6f6f43-4c1b-4c5e-9f3e-2d8c4f1a7b10",
	OrganizationID: "3d1f9a52-8e1b-4c3a-9b7e-5f2c6d8a0e41",
	Roles:          []string{"operator"},
}

const testRunID = "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b"

// fakeRuns records what the handler asked for and answers from fixed values.
type fakeRuns struct {
	state           contracts.RunState
	events          []contracts.SafeEvent
	err             error
	gotOrganization string
	gotRun          string
	gotAfter        int64
	gotLimit        int
	calls           int
}

func (fake *fakeRuns) RunState(_ context.Context, organizationID, runID string) (contracts.RunState, error) {
	fake.calls++
	fake.gotOrganization, fake.gotRun = organizationID, runID
	return fake.state, fake.err
}

func (fake *fakeRuns) RunEvents(_ context.Context, organizationID, runID string, afterEventID int64, limit int) ([]contracts.SafeEvent, error) {
	fake.calls++
	fake.gotOrganization, fake.gotRun, fake.gotAfter, fake.gotLimit = organizationID, runID, afterEventID, limit
	return fake.events, fake.err
}

// serve sends one request to handler, mounted on its pattern, as the given operator (nil: none).
func serve(t *testing.T, pattern string, handler http.Handler, target string, operator *contracts.OperatorContext) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(pattern, handler)
	request := httptest.NewRequest(http.MethodGet, target, nil)
	if operator != nil {
		request = request.WithContext(operatorcontext.WithOperator(request.Context(), *operator))
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

// expectError checks the shared error envelope and that nothing else is in the body.
func expectError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status %d, want %d: %s", recorder.Code, status, recorder.Body.String())
	}
	var envelope health.ErrorResponse
	if err := contracts.DecodeStrict(recorder.Body.Bytes(), &envelope); err != nil || envelope.Error.Code != code {
		t.Fatalf("body is not the %q envelope: %s", code, recorder.Body.String())
	}
}

func readFixture(t *testing.T, name string, target any) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(fixturesDirectory, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := contracts.DecodeStrict(content, target); err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return content
}

func TestRunStateServesTheX11StateOfTheOperatorsOrganization(t *testing.T) {
	var fixture contracts.RunState
	fixtureBytes := readFixture(t, "run-state.paused-allowance.json", &fixture)
	reader := &fakeRuns{state: fixture}
	// The organization in the query string is ignored: only the verified operator counts.
	recorder := serve(t, RunStateRoutePattern, RunStateHandler(reader),
		"/internal/runs/"+testRunID+"?organizationId=00000000-0000-4000-8000-000000000000", &testOperator)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var served contracts.RunState
	if err := contracts.DecodeStrict(recorder.Body.Bytes(), &served); err != nil || !reflect.DeepEqual(served, fixture) {
		t.Fatalf("served %+v, want the fixture", served)
	}
	var servedFields, fixtureFields map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &servedFields)
	_ = json.Unmarshal(fixtureBytes, &fixtureFields)
	if len(servedFields) != len(fixtureFields) {
		t.Fatalf("served fields %v, fixture fields %v", servedFields, fixtureFields)
	}
	if reader.gotOrganization != testOperator.OrganizationID || reader.gotRun != testRunID {
		t.Fatalf("read %s/%s", reader.gotOrganization, reader.gotRun)
	}
}

func TestRunReadsRejectWithoutDataBeforeAnyRead(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		operator *contracts.OperatorContext
		status   int
		code     string
	}{
		{"no operator", "/internal/runs/" + testRunID, nil, http.StatusUnauthorized, "unauthorized"},
		{"operator without organization", "/internal/runs/" + testRunID, &contracts.OperatorContext{UserID: testOperator.UserID}, http.StatusUnauthorized, "unauthorized"},
		{"malformed run id", "/internal/runs/not-a-run", &testOperator, http.StatusNotFound, "not_found"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			reader := &fakeRuns{state: contracts.RunState{RunID: testRunID}}
			expectError(t, serve(t, RunStateRoutePattern, RunStateHandler(reader), testCase.target, testCase.operator), testCase.status, testCase.code)
			expectError(t, serve(t, RunEventsRoutePattern, RunEventsHandler(reader), testCase.target+"/events", testCase.operator), testCase.status, testCase.code)
			expectError(t, serve(t, RunUsageRoutePattern, RunUsageHandler(nil), testCase.target+"/usage", testCase.operator), testCase.status, testCase.code)
			if reader.calls != 0 {
				t.Fatal("the reader was called for a rejected request")
			}
		})
	}
}

func TestRunReadsMapRepositoryErrors(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		// Another organization's run is not found, exactly like a run that does not exist.
		{repository.ErrNotFound, http.StatusNotFound, "not_found"},
		{repository.ErrUnavailable, http.StatusServiceUnavailable, "unavailable"},
		{errors.New("connection reset"), http.StatusServiceUnavailable, "unavailable"},
	}
	for _, testCase := range cases {
		reader := &fakeRuns{err: testCase.err, state: contracts.RunState{RunID: testRunID, PassportID: "leak"}}
		recorder := serve(t, RunStateRoutePattern, RunStateHandler(reader), "/internal/runs/"+testRunID, &testOperator)
		expectError(t, recorder, testCase.status, testCase.code)
		if strings.Contains(recorder.Body.String(), "leak") {
			t.Fatal("run data in an error body")
		}
		expectError(t, serve(t, RunEventsRoutePattern, RunEventsHandler(reader), "/internal/runs/"+testRunID+"/events", &testOperator), testCase.status, testCase.code)
	}
	expectError(t, serve(t, RunStateRoutePattern, RunStateHandler(nil), "/internal/runs/"+testRunID, &testOperator), http.StatusServiceUnavailable, "unavailable")
}

func TestRunEventsPagesByCursorWithX12Events(t *testing.T) {
	var denied, rejected contracts.SafeEvent
	readFixture(t, "safe-event.export-denied.json", &denied)
	readFixture(t, "safe-event.admission-rejected.json", &rejected)
	denied.EventID, rejected.EventID = "41", "42"
	reader := &fakeRuns{events: []contracts.SafeEvent{denied, rejected}}
	recorder := serve(t, RunEventsRoutePattern, RunEventsHandler(reader), "/internal/runs/"+testRunID+"/events?after=40&limit=2", &testOperator)
	var page RunEventPage
	if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &page) != nil {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	if page.NextCursor != "42" || !reflect.DeepEqual(page.Events, reader.events) || reader.gotAfter != 40 || reader.gotLimit != 2 {
		t.Fatalf("page %+v, read after %d limit %d", page, reader.gotAfter, reader.gotLimit)
	}

	// No new event: an empty list (never null) and the same cursor to poll with.
	reader.events = nil
	recorder = serve(t, RunEventsRoutePattern, RunEventsHandler(reader), "/internal/runs/"+testRunID+"/events?after=42", &testOperator)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"events":[]`) ||
		contracts.DecodeStrict(recorder.Body.Bytes(), &page) != nil || page.NextCursor != "42" || reader.gotLimit != defaultPageSize {
		t.Fatalf("empty page %s, limit %d", recorder.Body.String(), reader.gotLimit)
	}
}

func TestPageQueriesRejectInvalidValues(t *testing.T) {
	reader := &fakeRuns{}
	for _, query := range []string{"after=-1", "after=x", "after=1&after=2", "limit=0", "limit=501", "limit=ten", "after="} {
		expectError(t, serve(t, RunEventsRoutePattern, RunEventsHandler(reader), "/internal/runs/"+testRunID+"/events?"+query, &testOperator),
			http.StatusBadRequest, "bad_request")
	}
	for _, query := range []string{"cursor=", "cursor=v2.0.0.0", "cursor=v1.5.3.0", "cursor=v1.0.0.7", "cursor=v1.1.2", "cursor=v1.x.0.0", "limit=0", "cursor=v1.0.0.0&cursor=v1.0.0.0"} {
		expectError(t, serve(t, SecurityEventsRoutePattern, SecurityEventsHandler(nil), "/internal/security/events?"+query, &testOperator),
			http.StatusBadRequest, "bad_request")
		expectError(t, serve(t, SecurityAssessmentsRoutePattern, SecurityAssessmentsHandler(nil), "/internal/security/assessments?"+query, &testOperator),
			http.StatusBadRequest, "bad_request")
	}
	if reader.calls != 0 {
		t.Fatal("an invalid page reached the reader")
	}
}

func TestSecurityReadsNeedTheVerifiedOrganizationAndFailClosed(t *testing.T) {
	routes := []struct {
		pattern, target string
		handler         http.Handler
	}{
		{SecuritySummaryRoutePattern, "/internal/security/summary", SecuritySummaryHandler(nil)},
		{SecurityAssessmentsRoutePattern, "/internal/security/assessments", SecurityAssessmentsHandler(nil)},
		{SecurityEventsRoutePattern, "/internal/security/events", SecurityEventsHandler(nil)},
	}
	for _, route := range routes {
		expectError(t, serve(t, route.pattern, route.handler, route.target, nil), http.StatusUnauthorized, "unauthorized")
		// No database is never an empty summary or page.
		expectError(t, serve(t, route.pattern, route.handler, route.target, &testOperator), http.StatusServiceUnavailable, "unavailable")
	}
}

func TestWindowCursorRoundTripsAndAdvances(t *testing.T) {
	for _, cursor := range []windowCursor{{}, {Low: 900}, {Low: 900, High: 950, AfterID: 12}, {High: 950, AfterID: 3}} {
		parsed, ok := parseWindowCursor(cursor.String())
		if !ok || parsed != cursor {
			t.Fatalf("%v round-tripped to %v (%v)", cursor, parsed, ok)
		}
	}
	start := windowCursor{Low: 900}
	cases := []struct {
		name   string
		cursor windowCursor
		lastID int64
		more   bool
		high   uint64
		want   windowCursor
	}{
		{"full page keeps the window open", start, 17, true, 950, windowCursor{Low: 900, High: 950, AfterID: 17}},
		{"drained window starts the next at its end", start, 17, false, 950, windowCursor{Low: 950}},
		{"nothing final yet keeps the start", start, 0, false, 0, windowCursor{Low: 900}},
		// An open window that turns out empty still closes at its own end, never re-reading it.
		{"empty rest of an open window", windowCursor{Low: 900, High: 950, AfterID: 17}, 0, false, 0, windowCursor{Low: 950}},
		{"open window ignores a later end", windowCursor{Low: 900, High: 950, AfterID: 17}, 30, true, 990, windowCursor{Low: 900, High: 950, AfterID: 30}},
	}
	for _, testCase := range cases {
		if got := testCase.cursor.next(testCase.lastID, testCase.more, testCase.high); got != testCase.want {
			t.Fatalf("%s: got %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

func TestStoredRecordChecksRefuseWhatTheContractDoesNotAllow(t *testing.T) {
	runID, evaluationID := testRunID, "7a1b2c3d-4e5f-4a6b-9c7d-8e9f0a1b2c3d"
	live, fixture := "live", "fixture"
	semantic := AssessmentRecord{RunID: runID, EvaluationID: evaluationID, Boundary: "tool_result", ControlClass: "semantic",
		ControlID: "semantic_injection", Outcome: "block", AdmissionCatalogRevisionID: 1, EvaluatedCatalogRevisionID: 1,
		VerdictSource: &live, Verdict: &VerdictSummary{RiskCategory: "prompt_injection", Score: 0.9, ReasonCode: "semantic_injection_detected"}}
	if !validAssessment(semantic) {
		t.Fatal("a valid semantic record was refused")
	}
	deterministic := semantic
	deterministic.ControlClass, deterministic.VerdictSource, deterministic.Verdict = "deterministic", nil, nil
	if !validAssessment(deterministic) {
		t.Fatal("a valid deterministic record was refused")
	}
	broken := []func(*AssessmentRecord){
		func(record *AssessmentRecord) { record.VerdictSource = nil },
		func(record *AssessmentRecord) { record.VerdictSource = &fixture; record.ControlClass = "deterministic" },
		func(record *AssessmentRecord) {
			record.Verdict = &VerdictSummary{RiskCategory: "x", Score: 1.5, ReasonCode: "y"}
		},
		func(record *AssessmentRecord) { record.ControlID = "ignore previous instructions" },
		func(record *AssessmentRecord) { record.RunID = "run" },
		func(record *AssessmentRecord) { record.EvaluatedCatalogRevisionID = 0 },
	}
	for index, breakRecord := range broken {
		record := semantic
		breakRecord(&record)
		if validAssessment(record) {
			t.Fatalf("broken record %d was accepted", index)
		}
	}
	// A semantic check that made no model call (c1: no free-text arguments) has no source, verdict
	// or call, like the repository writer allows; with any of them, or another outcome, it is refused.
	unclassified := semantic
	unclassified.Outcome, unclassified.VerdictSource, unclassified.Verdict, unclassified.ReasonCode = "not_applicable", nil, nil, nil
	if !validAssessment(unclassified) {
		t.Fatal("a semantic not_applicable record without a verdict was refused")
	}
	for index, breakRecord := range []func(*AssessmentRecord){
		func(record *AssessmentRecord) { record.Outcome = "pass" },
		func(record *AssessmentRecord) {
			record.Verdict = &VerdictSummary{RiskCategory: "none", Score: 0, ReasonCode: "no_risk_found"}
		},
		func(record *AssessmentRecord) { call := evaluationID; record.SecurityModelCallID = &call },
	} {
		record := unclassified
		breakRecord(&record)
		if validAssessment(record) {
			t.Fatalf("broken unclassified record %d was accepted", index)
		}
	}
	// A stored verdict needs all three keys with values; a missing score is never served as 0.
	for _, stored := range []string{`{"risk_category":"none","reason_code":"no_risk_found"}`,
		`{"risk_category":"none","score":null,"reason_code":"no_risk_found"}`, `{"score":0.1,"reason_code":"no_risk_found"}`} {
		if _, err := decodeVerdict(stored); err == nil {
			t.Fatalf("incomplete verdict accepted: %s", stored)
		}
	}
	if verdict, err := decodeVerdict(`{"risk_category":"none","score":0,"reason_code":"no_risk_found"}`); err != nil || verdict.Score != 0 {
		t.Fatalf("a measured score of 0 was refused: %v", err)
	}
	// The verdict decodes strictly: a stored key beyond the decided schema is never passed on.
	if contracts.DecodeStrict([]byte(`{"risk_category":"a","score":0.1,"reason_code":"b","reasoning":"raw text"}`), &VerdictSummary{}) == nil {
		t.Fatal("an extra verdict key was accepted")
	}
}
