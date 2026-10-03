package agent

import (
	"context"
	"errors"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/model"
)

// RecordingCaller wraps the accounted model gateway for callers that choose their own call id,
// such as the semantic evaluator: it commits the GO-02 pre-dispatch record under that id before
// the call and records the outcome after it, so control assessments can reference the call.
type RecordingCaller struct {
	calls *budget.CallLog
	inner ModelCaller
	model string
}

// NewRecordingCaller returns a recording wrapper around the accounted caller.
func NewRecordingCaller(calls *budget.CallLog, inner ModelCaller, modelName string) (*RecordingCaller, error) {
	if calls == nil || inner == nil || modelName == "" {
		return nil, ErrInvalid
	}
	return &RecordingCaller{calls: calls, inner: inner, model: modelName}, nil
}

// Call records the dispatch, makes the accounted call and records its outcome. Without a
// dispatch record nothing is sent.
func (caller *RecordingCaller) Call(ctx context.Context, runID, callID string, request model.Request) (model.AccountedResult, error) {
	organizationID, err := caller.calls.RecordDispatchForRun(ctx, callID, runID, string(request.Purpose), caller.model)
	if err != nil {
		return model.AccountedResult{}, errors.Join(ErrRecording, err)
	}
	result, callErr := caller.inner.Call(ctx, runID, callID, request)
	outcome := budget.CallCompleted
	if callErr != nil {
		outcome = budget.CallFailed
		if result.UsageUnknown {
			outcome = budget.CallUsageUnknown
		}
	}
	recordContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if recordErr := caller.calls.RecordOutcome(recordContext, organizationID, callID, outcome); recordErr != nil && callErr == nil {
		return model.AccountedResult{}, ErrRecording
	}
	return result, callErr
}
