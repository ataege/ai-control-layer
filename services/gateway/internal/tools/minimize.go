package tools

import (
	"encoding/json"
	"errors"
	"fmt"
)

// MinimizedResult is what may enter the model context for one tool result (GO-23): the JSON of
// the tool's allowlisted fields only, and the untrusted free-text values in it, which the
// security checks (GO-74, GO-76) must inspect before the result becomes agent context.
type MinimizedResult struct {
	JSON          json.RawMessage
	UntrustedText []string
}

// failedResult is the only model-facing content of a failed outcome: the outcome and its reason.
type failedResult struct {
	Outcome    string `json:"outcome"`
	ReasonCode string `json:"reason_code"`
}

var errUnknownTool = errors.New("tools: no allowlist for this tool")

// MinimizeForModel applies the tool's allowlist to an effect result. The result is re-encoded
// through the tool's typed struct, so a field outside the allowlist is dropped even when the
// result arrives as a map or raw JSON. Protected values never appear: they are opaque
// references in the typed results.
func MinimizeForModel(tool string, result EffectResult) (MinimizedResult, error) {
	if result.Outcome == OutcomeFailed {
		encoded, err := json.Marshal(failedResult{Outcome: OutcomeFailed, ReasonCode: result.ReasonCode})
		if err != nil {
			return MinimizedResult{}, fmt.Errorf("tools: encode failed result: %w", err)
		}
		return MinimizedResult{JSON: encoded}, nil
	}
	if result.Outcome != OutcomeSucceeded {
		return MinimizedResult{}, fmt.Errorf("tools: unknown outcome %q", result.Outcome)
	}
	if result.ModelFacing == nil {
		return MinimizedResult{}, errors.New("tools: a succeeded result without content")
	}
	switch tool {
	case ToolReadInvoice:
		var invoice InvoiceResult
		if err := reencode(result.ModelFacing, &invoice); err != nil {
			return MinimizedResult{}, err
		}
		encoded, err := json.Marshal(invoice)
		if err != nil {
			return MinimizedResult{}, fmt.Errorf("tools: encode invoice result: %w", err)
		}
		minimized := MinimizedResult{JSON: encoded}
		if invoice.InternalNote != nil {
			minimized.UntrustedText = append(minimized.UntrustedText, invoice.InternalNote.Text)
		}
		return minimized, nil
	case ToolReadVendor:
		var vendor VendorResult
		if err := reencode(result.ModelFacing, &vendor); err != nil {
			return MinimizedResult{}, err
		}
		encoded, err := json.Marshal(vendor)
		if err != nil {
			return MinimizedResult{}, fmt.Errorf("tools: encode vendor result: %w", err)
		}
		// The vendor name is trusted demo data, not untrusted free text.
		return MinimizedResult{JSON: encoded}, nil
	case ToolCreateReport:
		var report ReportResult
		if err := reencode(result.ModelFacing, &report); err != nil {
			return MinimizedResult{}, err
		}
		encoded, err := json.Marshal(report)
		if err != nil {
			return MinimizedResult{}, fmt.Errorf("tools: encode report result: %w", err)
		}
		// References and Go-derived metadata only; the report content stays in the database.
		return MinimizedResult{JSON: encoded}, nil
	case ToolQueueReport:
		var queued QueueResult
		if err := reencode(result.ModelFacing, &queued); err != nil {
			return MinimizedResult{}, err
		}
		encoded, err := json.Marshal(queued)
		if err != nil {
			return MinimizedResult{}, fmt.Errorf("tools: encode queue result: %w", err)
		}
		return MinimizedResult{JSON: encoded}, nil
	default:
		return MinimizedResult{}, fmt.Errorf("%w: %s", errUnknownTool, tool)
	}
}

// reencode copies value into target through JSON, keeping only target's fields.
func reencode(value any, target any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("tools: encode result: %w", err)
	}
	if err := json.Unmarshal(encoded, target); err != nil {
		return fmt.Errorf("tools: result does not fit the allowlist: %w", err)
	}
	return nil
}
