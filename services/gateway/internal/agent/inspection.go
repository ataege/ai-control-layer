package agent

import (
	"context"
	"encoding/json"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/tools"
)

// InspectionOutcome is the result of the tool-result check before model context (Figure 10).
type InspectionOutcome string

const (
	// InspectionPass: the minimized result enters the context unchanged.
	InspectionPass InspectionOutcome = "pass"
	// InspectionRedacted: designated fields were masked by validated server operations.
	InspectionRedacted InspectionOutcome = "redacted"
	// InspectionBlocked: unsafe content is withheld; the context records only a withheld marker.
	InspectionBlocked InspectionOutcome = "blocked"
	// InspectionPause: the guard could not decide (failure, exhausted security allowance). Nothing
	// is released and the run pauses.
	InspectionPause InspectionOutcome = "pause"
)

// Inspection is what may enter the model context for one tool result. Content is the permitted
// JSON for pass and redacted, a withheld marker for blocked, and empty for pause.
type Inspection struct {
	Outcome InspectionOutcome
	Content json.RawMessage
	Reason  contracts.ReasonCode
}

// ResultInspector checks a minimized tool result before it becomes model context (GO-76).
type ResultInspector interface {
	Inspect(ctx context.Context, run Run, tool string, result tools.MinimizedResult) (Inspection, error)
}

// InterimUntrustedTextGuard is the INTERIM inspector used until the hybrid tool-result check
// (internal/security, GO-76) is wired. It is not a protection: it fails closed by pausing the run
// for any result that carries untrusted free text, and passes only results that carry none.
type InterimUntrustedTextGuard struct{}

// Inspect pauses on untrusted text or invalid JSON and passes everything else unchanged.
func (InterimUntrustedTextGuard) Inspect(_ context.Context, _ Run, _ string, result tools.MinimizedResult) (Inspection, error) {
	if len(result.UntrustedText) > 0 || !json.Valid(result.JSON) {
		return Inspection{Outcome: InspectionPause, Reason: contracts.ReasonSecurityEvaluatorUnavailable}, nil
	}
	return Inspection{Outcome: InspectionPass, Content: append(json.RawMessage(nil), result.JSON...)}, nil
}
