package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"starter/services/gateway/internal/admission"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/operatorcontext"
)

// fakeJudgeAdmitter stands in for admission.Admitter's AdmitJudge.
type fakeJudgeAdmitter struct {
	passport contracts.Passport
	err      error
	calls    int
	operator contracts.OperatorContext
}

func (admitter *fakeJudgeAdmitter) AdmitJudge(_ context.Context, operator contracts.OperatorContext, _ contracts.StartRunRequest) (contracts.Passport, error) {
	admitter.calls++
	admitter.operator = operator
	return admitter.passport, admitter.err
}

func postJudgeRun(t *testing.T, handler http.Handler, tokenID, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/internal/judge-runs", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+testServiceToken)
	request.Header.Set(operatorcontext.HeaderName, signOperatorContext(t, testOperator, tokenID))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// POST /internal/judge-runs answers like POST /internal/runs, but through AdmitJudge only.
func TestJudgeRunRouteAdmitsThroughTheJudgeAdmission(t *testing.T) {
	judges := &fakeJudgeAdmitter{passport: contracts.Passport{
		RunID: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b", PassportID: "1f2e3d4c-5b6a-4789-8a0b-c1d2e3f4a5b6"}}
	agents := &fakeAdmitter{}
	handler := newHandler(t, Dependencies{Admitter: agents, Judges: judges})

	recorder := postJudgeRun(t, handler, "judge-1", validBody)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response contracts.StartRunResponse
	decodeStrict(t, recorder, &response)
	if response.RunID != judges.passport.RunID || response.PassportID != judges.passport.PassportID ||
		judges.calls != 1 || agents.calls != 0 || judges.operator.OrganizationID != testOperator.OrganizationID {
		t.Fatalf("response %+v; judge calls %d, agent calls %d", response, judges.calls, agents.calls)
	}

	// The same strict body: an unknown field never reaches admission.
	assertError(t, postJudgeRun(t, handler, "judge-2", `{"template":"reconcile_atlas_v1","invoiceIds":["invoice_A01"],"destination":"vendor_Atlas","organizationId":"x"}`),
		http.StatusBadRequest, "bad_request")
	if judges.calls != 1 {
		t.Errorf("an invalid body reached admission")
	}

	// The same answers: a rejection keeps its reason code.
	rejected := &fakeJudgeAdmitter{err: &admission.Rejection{Code: contracts.ReasonResourceOutOfScope, Message: "an invoice in invoiceIds is not available to this organization"}}
	assertError(t, postJudgeRun(t, newHandler(t, Dependencies{Judges: rejected}), "judge-3", validBody),
		http.StatusBadRequest, "resource_out_of_scope")
	unavailable := &fakeJudgeAdmitter{err: admission.ErrUnavailable}
	assertError(t, postJudgeRun(t, newHandler(t, Dependencies{Judges: unavailable}), "judge-4", validBody),
		http.StatusServiceUnavailable, "decision_unavailable")
}
