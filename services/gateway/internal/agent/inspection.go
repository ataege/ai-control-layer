package agent

import (
	"context"
	"encoding/json"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/security"
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
	// Evidence is the hybrid check's safe record (control decisions, metered security calls), for
	// the control assessment and timing records (GO-80). It holds no inspected text.
	Evidence *security.ToolResultInspection
}

// ResultInspector checks a minimized tool result before it becomes model context (GO-76).
type ResultInspector interface {
	Inspect(ctx context.Context, run Run, tool string, result tools.MinimizedResult, settings security.Settings) (Inspection, error)
}
