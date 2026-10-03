package tools

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMinimizeDropsFieldsOutsideTheAllowlist(t *testing.T) {
	// A result that carries extra fields, at the top level and inside the note.
	raw := map[string]any{
		"invoice_id": "invoice_A01", "version": 1, "vendor_id": "vendor_Atlas",
		"external_reference": "INV104", "currency": "EUR", "total_minor_units": 125000,
		"issued_on": "2026-09-01", "due_on": "2026-10-31",
		"internal_note":                map[string]any{"text": "note", "classification": "internal_only", "author": "x"},
		"registered_reporting_address": "reports@atlas.example.com",
		"bank_account":                 "GB82 WEST 1234 5698 7654 32",
	}
	minimized, err := MinimizeForModel(ToolReadInvoice, EffectResult{Outcome: OutcomeSucceeded, ModelFacing: raw})
	if err != nil {
		t.Fatalf("MinimizeForModel: %v", err)
	}
	serialized := string(minimized.JSON)
	for _, forbidden := range []string{"registered_reporting_address", "reports@atlas", "bank_account", "GB82", "author"} {
		if strings.Contains(serialized, forbidden) {
			t.Errorf("minimized result still holds %q: %s", forbidden, serialized)
		}
	}
	if !strings.Contains(serialized, `"classification":"internal_only"`) {
		t.Errorf("the note lost its classification: %s", serialized)
	}
	if len(minimized.UntrustedText) != 1 || minimized.UntrustedText[0] != "note" {
		t.Errorf("untrusted text = %v, want the note text for the security checks", minimized.UntrustedText)
	}
}

func TestMinimizeReturnsOnlyOutcomeAndReasonForAFailure(t *testing.T) {
	minimized, err := MinimizeForModel(ToolReadInvoice, EffectResult{
		Outcome: OutcomeFailed, ReasonCode: ReasonResourceOutOfScope, ModelFacing: map[string]any{"leak": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(minimized.JSON, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 || decoded["outcome"] != "failed" || decoded["reason_code"] != ReasonResourceOutOfScope {
		t.Fatalf("failed result = %s", minimized.JSON)
	}
}

func TestMinimizeFailsClosedForUnknownToolsAndOutcomes(t *testing.T) {
	if _, err := MinimizeForModel("run_shell", EffectResult{Outcome: OutcomeSucceeded, ModelFacing: map[string]any{}}); !errors.Is(err, errUnknownTool) {
		t.Errorf("unknown tool: err = %v", err)
	}
	if _, err := MinimizeForModel(ToolReadInvoice, EffectResult{Outcome: "unknown"}); err == nil {
		t.Error("unknown outcome was minimized instead of refused")
	}
}
