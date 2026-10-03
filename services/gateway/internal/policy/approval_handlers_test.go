package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/operatorcontext"
)

// recordingDecider records whether a decision reached it.
type recordingDecider struct {
	calls  int
	choice ApprovalChoice
	err    error
}

func (decider *recordingDecider) Decide(_ context.Context, _ contracts.OperatorContext, _ string, choice ApprovalChoice) (ApprovalResult, error) {
	decider.calls++
	decider.choice = choice
	if decider.err != nil {
		return ApprovalResult{}, decider.err
	}
	return ApprovalResult{ApprovalID: "approval", ActionID: "action", RunID: "run", Decision: choice}, nil
}

func approvalRequest(body string, withOperator bool) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/internal/actions/"+testActionID+"/approval", strings.NewReader(body))
	request.SetPathValue("actionId", testActionID)
	if withOperator {
		request = request.WithContext(operatorcontext.WithOperator(request.Context(),
			contracts.OperatorContext{UserID: testActionID, OrganizationID: testOrganizationID, Roles: []string{"reviewer"}}))
	}
	return request
}

func TestApprovalHandlerAcceptsOnlyTheDecision(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		withOperator bool
		deciderErr   error
		wantStatus   int
		wantDecided  bool
	}{
		{"approve", `{"decision":"approve"}`, true, nil, http.StatusOK, true},
		{"reject", `{"decision":"reject"}`, true, nil, http.StatusOK, true},
		{"a body that carries a payload", `{"decision":"approve","recipient":"attacker@example.com"}`, true, nil, http.StatusBadRequest, false},
		{"a replacement content", `{"decision":"approve","content":"other"}`, true, nil, http.StatusBadRequest, false},
		{"no decision", `{}`, true, nil, http.StatusBadRequest, false},
		{"not JSON", `approve`, true, nil, http.StatusBadRequest, false},
		{"unknown decision value", `{"decision":"maybe"}`, true, ErrApprovalInvalid, http.StatusBadRequest, true},
		{"no operator context", `{"decision":"approve"}`, false, nil, http.StatusUnauthorized, false},
		{"not a reviewer", `{"decision":"approve"}`, true, ErrNotReviewer, http.StatusForbidden, true},
		{"expired", `{"decision":"approve"}`, true, ErrApprovalExpired, http.StatusConflict, true},
		{"store failure", `{"decision":"approve"}`, true, ErrApprovalUnavailable, http.StatusServiceUnavailable, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			decider := &recordingDecider{err: testCase.deciderErr}
			recorder := httptest.NewRecorder()
			ApprovalHandler(decider).ServeHTTP(recorder, approvalRequest(testCase.body, testCase.withOperator))
			if recorder.Code != testCase.wantStatus || (decider.calls == 1) != testCase.wantDecided {
				t.Fatalf("status %d with %d decisions; want %d, decided %v (body %s)", recorder.Code, decider.calls, testCase.wantStatus, testCase.wantDecided, recorder.Body)
			}
		})
	}
}
