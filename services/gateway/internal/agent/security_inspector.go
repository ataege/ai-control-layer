package agent

import (
	"context"
	"encoding/json"
	"errors"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/security"
	"starter/services/gateway/internal/tools"
)

// ToolResultInspector is the hybrid tool-result check (*security.Inspector).
type ToolResultInspector interface {
	InspectToolResult(ctx context.Context, input security.ToolResultInput, settings security.Settings) (security.ToolResultInspection, error)
}

// SecurityInspector connects the loop to the hybrid tool-result check of Figure 10 (GO-76): the
// active catalog's deterministic content and signature rules, then the metered semantic check on
// the untrusted note. Only the inspection's permitted JSON enters the model context.
type SecurityInspector struct {
	inspector ToolResultInspector
}

// NewSecurityInspector returns the production inspector.
func NewSecurityInspector(inspector ToolResultInspector) (*SecurityInspector, error) {
	if inspector == nil {
		return nil, ErrInvalid
	}
	return &SecurityInspector{inspector: inspector}, nil
}

// resultSource holds the trusted identity fields of a minimized result.
type resultSource struct {
	InvoiceID    string `json:"invoice_id"`
	VendorID     string `json:"vendor_id"`
	Version      int64  `json:"version"`
	InternalNote *struct {
		Text           string `json:"text"`
		Classification string `json:"classification"`
	} `json:"internal_note"`
}

// Inspect, with the settings of the catalog snapshot read for this step, builds the tool-result input with the trusted source of every untrusted value and maps
// the outcome. Any untrusted text it cannot place, any settings or guard failure pauses the run.
func (inspector *SecurityInspector) Inspect(ctx context.Context, run Run, tool string, result tools.MinimizedResult, settings security.Settings) (Inspection, error) {
	pause := Inspection{Outcome: InspectionPause, Reason: contracts.ReasonSecurityEvaluatorUnavailable}
	var source resultSource
	if json.Unmarshal(result.JSON, &source) != nil {
		return pause, nil
	}
	input := security.ToolResultInput{RunID: run.RunID, Tool: tool, ResultJSON: result.JSON}
	switch tool {
	case tools.ToolReadInvoice:
		input.Source = security.SourceRef{SourceID: source.InvoiceID, Version: source.Version}
		if source.InternalNote != nil {
			input.Untrusted = append(input.Untrusted, security.UntrustedPath{
				Path: []string{"internal_note", "text"}, Name: security.FieldInternalNote,
				Source: security.SourceRef{SourceID: source.InvoiceID, Version: source.Version, Classification: source.InternalNote.Classification},
			})
		}
	case tools.ToolReadVendor:
		input.Source = security.SourceRef{SourceID: source.VendorID, Version: source.Version}
	default:
		input.Source = security.SourceRef{SourceID: tool}
	}
	// Every untrusted text the minimizer reported must be on a path the semantic check covers.
	if len(input.Untrusted) != len(result.UntrustedText) {
		return pause, nil
	}
	inspection, err := inspector.inspector.InspectToolResult(ctx, input, settings)
	evidence := inspection
	if err != nil || inspection.Outcome == security.ResultPaused {
		reason := contracts.ReasonCode(inspection.ReasonCode)
		if !reason.Valid() {
			reason = contracts.ReasonSecurityEvaluatorUnavailable
		}
		return Inspection{Outcome: InspectionPause, Reason: reason, Evidence: &evidence}, nil
	}
	reason := contracts.ReasonCode(inspection.ReasonCode)
	if inspection.ReasonCode != "" && !reason.Valid() {
		return Inspection{Outcome: InspectionPause, Reason: contracts.ReasonSecurityEvaluatorUnavailable, Evidence: &evidence}, nil
	}
	switch inspection.Outcome {
	case security.ResultPass:
		return Inspection{Outcome: InspectionPass, Content: inspection.ResultJSON, Evidence: &evidence}, nil
	case security.ResultRedacted:
		return Inspection{Outcome: InspectionRedacted, Content: inspection.ResultJSON, Reason: reason, Evidence: &evidence}, nil
	case security.ResultBlocked:
		content := inspection.ResultJSON
		if len(content) == 0 {
			// Withheld whole: the context records only that the result was withheld, and why.
			content, err = json.Marshal(struct {
				Withheld   bool   `json:"withheld"`
				ReasonCode string `json:"reason_code"`
			}{Withheld: true, ReasonCode: string(reason)})
			if err != nil {
				return pause, nil
			}
		}
		return Inspection{Outcome: InspectionBlocked, Content: content, Reason: reason, Evidence: &evidence}, nil
	default:
		return Inspection{Outcome: InspectionPause, Reason: contracts.ReasonSecurityEvaluatorUnavailable, Evidence: &evidence}, errors.New("unknown inspection outcome")
	}
}
