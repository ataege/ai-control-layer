package scenario

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/operatorcontext"
	"starter/services/gateway/internal/reads"
)

// TestSecurityReadsServeTheRecordsOfARealRun reads the security records a real run writes: the
// story through the production chain stores, among others, the semantic not_applicable records the
// action-proposal check writes for constrained arguments (no verdict, no source, no call). The
// assessment page, the event page and the summary must serve them, not refuse the organization.
func TestSecurityReadsServeTheRecordsOfARealRun(t *testing.T) {
	world := openStory(t, nil)
	world.runQueuedJob(t)

	get := func(pattern string, handler http.Handler, target string) *httptest.ResponseRecorder {
		t.Helper()
		mux := http.NewServeMux()
		mux.Handle(pattern, handler)
		request := httptest.NewRequest(http.MethodGet, target, nil)
		request = request.WithContext(operatorcontext.WithOperator(request.Context(), world.operator))
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		return recorder
	}

	// Page the assessments until the run's records are final (window cursor).
	var records []reads.AssessmentRecord
	cursor := ""
	deadline := time.Now().Add(20 * time.Second)
	unclassified := 0
	for unclassified == 0 {
		target := "/internal/security/assessments?limit=500"
		if cursor != "" {
			target += "&cursor=" + cursor
		}
		recorder := get(reads.SecurityAssessmentsRoutePattern, reads.SecurityAssessmentsHandler(world.pool), target)
		var page reads.AssessmentPage
		if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &page) != nil {
			t.Fatalf("assessments %d: %s", recorder.Code, recorder.Body.String())
		}
		records = append(records, page.Records...)
		cursor = page.NextCursor
		unclassified = 0
		for _, record := range records {
			if record.ControlClass == "semantic" && record.Outcome == "not_applicable" && record.VerdictSource == nil {
				unclassified++
			}
		}
		if unclassified == 0 && time.Now().After(deadline) {
			t.Fatalf("no unclassified semantic record among %d records", len(records))
		}
		if unclassified == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	recorder := get(reads.SecurityEventsRoutePattern, reads.SecurityEventsHandler(world.pool), "/internal/security/events?limit=500")
	var events reads.SecurityEventPage
	if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &events) != nil {
		t.Fatalf("events %d: %s", recorder.Code, recorder.Body.String())
	}
	recorder = get(reads.SecuritySummaryRoutePattern, reads.SecuritySummaryHandler(world.pool), "/internal/security/summary")
	var summary reads.SecuritySummary
	if recorder.Code != http.StatusOK || contracts.DecodeStrict(recorder.Body.Bytes(), &summary) != nil {
		t.Fatalf("summary %d: %s", recorder.Code, recorder.Body.String())
	}
	unclassifiedCount := int64(0)
	for _, count := range summary.Assessments {
		if count.ControlClass == "semantic" && count.Outcome == "not_applicable" && count.VerdictSource == nil {
			unclassifiedCount += count.Count
		}
	}
	if unclassifiedCount == 0 {
		t.Fatalf("the summary does not count the unclassified records: %+v", summary.Assessments)
	}
	t.Logf("evidence GO-83: a real run's %d assessments (%d semantic not_applicable without a verdict) served 200; events and summary 200",
		len(records), unclassified)
}
