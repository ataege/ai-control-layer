package security_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/security"
)

// Evidence X-97, the semantic false-negative boundary. The security model is replaced by a labelled
// fixture that always answers "no risk" (score 0): it models a semantic false negative. It says
// nothing about detection quality. The gate is Worker 3's real policy.Gate; its passport, recorder
// and relationship readers are test doubles holding the Atlas scenario.

const (
	boundaryOrganizationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	boundaryRunID          = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	boundaryActionID       = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	vendorReportID         = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	internalReportID       = "12121212-1212-4121-8121-121212121212"
	registeredRecipient    = "recipient:" + boundaryRunID + ":vendor_atlas"
)

// permissiveProvider is a labelled provider double: every verdict is "none", score 0.
type permissiveProvider struct {
	mutex sync.Mutex
	calls int
}

func (provider *permissiveProvider) Chat(context.Context, model.Request) (model.Result, error) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	provider.calls++
	input, output := int64(40), int64(12)
	return model.Result{
		Message: model.Message{Role: "assistant", Content: `{"risk_category":"none","score":0,"reason_code":"no_risk_found"}`},
		Usage:   model.Usage{InputTokens: &input, OutputTokens: &output},
	}, nil
}

// boundaryLedger is a labelled in-memory allowance double with ample room.
type boundaryLedger struct{}

func (boundaryLedger) Reserve(_ context.Context, _, _, _ string, tokens int64) (budget.Reservation, error) {
	return budget.Reservation{Tokens: tokens}, nil
}
func (boundaryLedger) MarkUnknown(context.Context, string, string) error { return nil }
func (boundaryLedger) Settle(_ context.Context, _, _ string, input, output int64) (budget.Settlement, error) {
	return budget.Settlement{ActualTokens: input + output}, nil
}
func (boundaryLedger) Snapshot(context.Context, string) (budget.Snapshot, error) {
	return budget.Snapshot{}, nil
}

type atlasScopes struct{}

func (atlasScopes) LoadScope(context.Context, policy.RunIdentity) (policy.PassportScope, error) {
	return policy.PassportScope{
		OrganizationID:        boundaryOrganizationID,
		RunID:                 boundaryRunID,
		PassportID:            "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		AllowedTools:          []policy.ToolName{policy.ToolReadInvoice, policy.ToolReadVendor, policy.ToolCreateReport, policy.ToolQueueReport},
		AllowedInvoiceIDs:     []string{"invoice_A01", "invoice_A02"},
		AllowedTemplates:      []string{policy.TemplateInternalInvestigation, policy.TemplateVendorReconciliation},
		RecipientReferences:   []string{registeredRecipient},
		ApprovalRequiredTools: []policy.ToolName{policy.ToolQueueReport},
		ExpiresAt:             time.Now().Add(15 * time.Minute),
	}, nil
}
func (atlasScopes) ActiveCatalogRevision(context.Context) (int64, error) { return 3, nil }

type decisionRecorder struct{ decisions []policy.Decision }

func (*decisionRecorder) StoreAction(context.Context, policy.StoredAction) error { return nil }
func (recorder *decisionRecorder) RecordDecision(_ context.Context, _ policy.RunIdentity, decision policy.Decision) error {
	recorder.decisions = append(recorder.decisions, decision)
	return nil
}

// atlasRelationships holds the scenario's stored lineage: the internal report is Internal only.
type atlasRelationships struct{}

func (atlasRelationships) VendorLinkedToInvoices(_ context.Context, _, vendorID string, _ []string) (bool, error) {
	return vendorID == "vendor_atlas", nil
}
func (atlasRelationships) ReportExport(_ context.Context, _, _, reportID string) (policy.ExportVerdict, error) {
	switch reportID {
	case internalReportID:
		return policy.ExportVerdict{Found: true, ReasonCode: policy.ReasonReportExportRestricted, AlternativeTemplate: policy.TemplateVendorReconciliation}, nil
	case vendorReportID:
		return policy.ExportVerdict{Found: true, Allowed: true}, nil
	}
	return policy.ExportVerdict{}, nil
}

// stubFreezer is a labelled review-freezer double: it freezes nothing real, so approval requests can
// be observed (GO-43 denies an approval request when no freezer is configured).
type stubFreezer struct{}

func (stubFreezer) Freeze(_ context.Context, _ policy.RunIdentity, action policy.StoredAction, _ policy.PassportScope) (policy.FrozenReview, error) {
	return policy.FrozenReview{PayloadID: "00000000-0000-4000-8000-000000000001", ReportID: action.ActionID}, nil
}

// gateEvaluator adapts security.EvaluateAction to policy.ActionEvaluator the way the README
// describes: no_objection keeps the deterministic decision, block denies, pause is an error.
type gateEvaluator struct {
	inspector *security.Inspector
	settings  security.Settings
}

func (evaluator gateEvaluator) EvaluateAction(ctx context.Context, run policy.RunIdentity, action policy.StoredAction) (policy.ActionCheck, error) {
	assessment, err := evaluator.inspector.EvaluateAction(ctx, security.ActionInput{
		RunID: run.RunID, ActionID: action.ActionID, Tool: string(action.Tool), CanonicalArguments: action.CanonicalArguments,
	}, evaluator.settings)
	switch {
	case err != nil:
		return policy.ActionCheck{Records: assessment.Records}, err
	case assessment.Decision == security.ActionBlock:
		return policy.ActionCheck{Outcome: policy.OutcomeDeny, ReasonCode: policy.ReasonCode(assessment.ReasonCode), Records: assessment.Records}, nil
	case assessment.Decision == security.ActionNoObjection:
		return policy.ActionCheck{Outcome: policy.OutcomeAllow, Records: assessment.Records}, nil
	}
	return policy.ActionCheck{Records: assessment.Records}, security.ErrEvaluatorUnavailable
}

// sampleSettings builds the settings the way GO-72 will: the sample policy's content and the
// committed feed with its digest.
func sampleSettings(t *testing.T) security.Settings {
	t.Helper()
	feed, err := os.ReadFile(filepath.Join("../../../../config", "attack-signatures.json"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(feed)
	content := []byte(`{"controls":{
	  "secret_pattern":{"enabled":true,"mode":"redact","boundaries":["model_input","tool_result"]},
	  "semantic_injection":{"enabled":true,"mode":"block","threshold":0.75,"boundaries":["model_input","tool_result","action_proposal"]},
	  "signature_match":{"enabled":true,"boundaries":["model_input","tool_result","action_proposal"]}},
	  "signatures":{"path":"attack-signatures.json","revision":"feed_v1","disabled_rules":[]}}`)
	settings, err := security.SettingsFromCatalog(3, content, feed, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	return settings
}

// Each hostile note's instruction, if a semantic false negative let the agent obey it, becomes a
// proposal that the deterministic gate must still deny, with the reason the fixture names.
func TestSemanticFalseNegativeStillDeniedDeterministically(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("../../../../fixtures", "hostile-notes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var notes struct {
		Notes []struct {
			ID                          string `json:"id"`
			Attack                      string `json:"attack"`
			DeterministicReasonIfObeyed string `json:"deterministic_reason_if_obeyed"`
		} `json:"notes"`
	}
	if err := json.Unmarshal(raw, &notes); err != nil {
		t.Fatal(err)
	}
	// The action an agent obeying each note would propose.
	obeyed := map[string]policy.Proposal{
		"redirect_to_out_of_scope_record": {Tool: "read_invoice", RawArguments: json.RawMessage(`{"invoice_id":"invoice_B01"}`)},
		"redirect_to_other_recipient": {Tool: "queue_report", RawArguments: json.RawMessage(
			`{"report_id":"` + vendorReportID + `","recipient_reference":"recipient:` + boundaryRunID + `:settlement_desk"}`)},
		"include_internal_information_in_vendor_message": {Tool: "queue_report", RawArguments: json.RawMessage(
			`{"report_id":"` + internalReportID + `","recipient_reference":"` + registeredRecipient + `"}`)},
	}
	run := policy.RunIdentity{OrganizationID: boundaryOrganizationID, RunID: boundaryRunID}
	for _, note := range notes.Notes {
		t.Run(note.ID, func(t *testing.T) {
			proposal, known := obeyed[note.Attack]
			if !known {
				t.Fatalf("no obeyed proposal for attack %q", note.Attack)
			}
			proposal.ActionID, proposal.StepNumber, proposal.IdempotencyKey = boundaryActionID, 1, "boundary:step-1"
			provider := &permissiveProvider{}
			caller, err := model.NewAccountedCaller(provider, boundaryLedger{}, model.DefaultAccountingSettings())
			if err != nil {
				t.Fatal(err)
			}
			evaluator, err := security.NewSemanticEvaluator(caller, security.EvaluatorOptions{
				Model: "fixture", ContextTokens: security.MinEvaluatorContextTokens, Source: security.VerdictFixture,
			})
			if err != nil {
				t.Fatal(err)
			}
			recorder := &decisionRecorder{}
			gate := policy.NewGate(atlasScopes{}, recorder, atlasRelationships{},
				gateEvaluator{inspector: security.NewInspector(evaluator), settings: sampleSettings(t)})
			decision := gate.Evaluate(context.Background(), run, proposal)
			if decision.Outcome != policy.OutcomeDeny || string(decision.ReasonCode) != note.DeterministicReasonIfObeyed {
				t.Fatalf("decision = %s/%s, want deny/%s", decision.Outcome, decision.ReasonCode, note.DeterministicReasonIfObeyed)
			}
			// Deterministic checks run first, so the permissive verdict is never even asked for.
			if provider.calls != 0 || len(recorder.decisions) != 1 {
				t.Fatalf("security calls = %d, recorded decisions = %d", provider.calls, len(recorder.decisions))
			}
			t.Logf("evidence X-97: %s obeyed as %s with a fixture verdict of score 0 -> %s %s; security calls 0; no allow recorded",
				note.ID, proposal.Tool, decision.Outcome, decision.ReasonCode)
		})
	}

	// Control: a permitted read does reach the fixture evaluator, whose "no risk" adds nothing.
	provider := &permissiveProvider{}
	caller, _ := model.NewAccountedCaller(provider, boundaryLedger{}, model.DefaultAccountingSettings())
	evaluator, _ := security.NewSemanticEvaluator(caller, security.EvaluatorOptions{Model: "fixture", ContextTokens: security.MinEvaluatorContextTokens, Source: security.VerdictFixture})
	gate := policy.NewGate(atlasScopes{}, &decisionRecorder{}, atlasRelationships{}, gateEvaluator{inspector: security.NewInspector(evaluator), settings: sampleSettings(t)}).
		WithReviewFreezer(stubFreezer{})
	permitted := policy.Proposal{ActionID: boundaryActionID, StepNumber: 1, IdempotencyKey: "boundary:step-1", Tool: "read_invoice", RawArguments: json.RawMessage(`{"invoice_id":"invoice_A01"}`)}
	if decision := gate.Evaluate(context.Background(), run, permitted); decision.Outcome != policy.OutcomeAllow || provider.calls != 1 {
		t.Fatalf("control: decision = %s/%s, security calls = %d", decision.Outcome, decision.ReasonCode, provider.calls)
	}
	// A permissive verdict never lifts review: the vendor report to the registered recipient still needs approval.
	review := policy.Proposal{ActionID: boundaryActionID, StepNumber: 2, IdempotencyKey: "boundary:step-2", Tool: "queue_report",
		RawArguments: json.RawMessage(`{"report_id":"` + vendorReportID + `","recipient_reference":"` + registeredRecipient + `"}`)}
	if decision := gate.Evaluate(context.Background(), run, review); decision.Outcome != policy.OutcomeApprovalRequired || provider.calls != 2 {
		t.Fatalf("review: decision = %s/%s, security calls = %d", decision.Outcome, decision.ReasonCode, provider.calls)
	}
}
