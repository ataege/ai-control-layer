package policy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

const (
	testOrganizationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testRunID          = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	testPassportID     = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	testActionID       = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	testRecipient      = "recipient:" + testRunID + ":vendor_atlas"
)

// fakeScopes returns a fixed scope and revision, or the configured errors.
type fakeScopes struct {
	scope       PassportScope
	scopeError  error
	revision    int64
	revisionErr error
}

func (scopes *fakeScopes) LoadScope(context.Context, RunIdentity) (PassportScope, error) {
	return scopes.scope, scopes.scopeError
}

func (scopes *fakeScopes) ActiveCatalogRevision(context.Context) (int64, error) {
	return scopes.revision, scopes.revisionErr
}

// fakeRecorder keeps the call order so a test can prove the action is stored before the decision.
type fakeRecorder struct {
	calls        []string
	storedAction *StoredAction
	decisions    []Decision
	storeError   error
	recordError  error
}

func (recorder *fakeRecorder) StoreAction(_ context.Context, action StoredAction) error {
	recorder.calls = append(recorder.calls, "store")
	if recorder.storeError != nil {
		return recorder.storeError
	}
	recorder.storedAction = &action
	return nil
}

func (recorder *fakeRecorder) RecordDecision(_ context.Context, _ RunIdentity, decision Decision) error {
	recorder.calls = append(recorder.calls, "decision")
	recorder.decisions = append(recorder.decisions, decision)
	return recorder.recordError
}

// fakeEvaluator returns a fixed semantic verdict and counts its calls.
type fakeEvaluator struct {
	outcome Outcome
	reason  ReasonCode
	err     error
	calls   int
}

func (evaluator *fakeEvaluator) EvaluateAction(context.Context, RunIdentity, StoredAction) (Outcome, ReasonCode, error) {
	evaluator.calls++
	return evaluator.outcome, evaluator.reason, evaluator.err
}

func atlasScope() PassportScope {
	return PassportScope{
		OrganizationID:        testOrganizationID,
		RunID:                 testRunID,
		PassportID:            testPassportID,
		AllowedTools:          []ToolName{ToolReadInvoice, ToolReadVendor, ToolCreateReport, ToolQueueReport},
		AllowedInvoiceIDs:     []string{"invoice_A01", "invoice_A02"},
		AllowedTemplates:      []string{TemplateInternalInvestigation, TemplateVendorReconciliation},
		RecipientReferences:   []string{testRecipient},
		ApprovalRequiredTools: []ToolName{ToolQueueReport},
		ExpiresAt:             time.Now().Add(15 * time.Minute),
	}
}

func testRun() RunIdentity { return RunIdentity{OrganizationID: testOrganizationID, RunID: testRunID} }

func proposal(tool, rawArguments string) Proposal {
	return Proposal{ActionID: testActionID, StepNumber: 1, IdempotencyKey: "run-b:step-1", Tool: tool, RawArguments: json.RawMessage(rawArguments)}
}

func newTestGate(scope PassportScope) (*Gate, *fakeScopes, *fakeRecorder) {
	scopes := &fakeScopes{scope: scope, revision: 3}
	recorder := &fakeRecorder{}
	return NewGate(scopes, recorder, nil), scopes, recorder
}

func TestGateDecisions(t *testing.T) {
	cases := []struct {
		name        string
		tool        string
		arguments   string
		adjustScope func(*PassportScope)
		wantOutcome Outcome
		wantReason  ReasonCode
		wantStored  bool
	}{
		{"permitted read", "read_invoice", `{"invoice_id":"invoice_A01"}`, nil, OutcomeAllow, "", true},
		{"permitted internal report", "create_report", `{"template":"internal_investigation_v1","source_invoice_ids":["invoice_A01","invoice_A02"]}`, nil, OutcomeAllow, "", true},
		{"queue_report needs approval", "queue_report", `{"report_id":"eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee","recipient_reference":"` + testRecipient + `"}`, nil, OutcomeApprovalRequired, ReasonApprovalRequired, true},
		{"unregistered tool", "send_email", `{"to":"x"}`, nil, OutcomeDeny, ReasonToolNotRegistered, false},
		{"unknown field", "read_invoice", `{"invoice_id":"invoice_A01","extra":1}`, nil, OutcomeDeny, ReasonInvalidArguments, false},
		{"missing field", "create_report", `{"template":"internal_investigation_v1"}`, nil, OutcomeDeny, ReasonInvalidArguments, false},
		{"wrong type", "read_invoice", `{"invoice_id":7}`, nil, OutcomeDeny, ReasonInvalidArguments, false},
		{"tool missing from passport", "read_vendor", `{"vendor_id":"vendor_atlas"}`, func(scope *PassportScope) { scope.AllowedTools = []ToolName{ToolReadInvoice} }, OutcomeDeny, ReasonToolNotAllowed, true},
		{"invoice outside the passport", "read_invoice", `{"invoice_id":"invoice_B01"}`, nil, OutcomeDeny, ReasonResourceOutOfScope, true},
		{"report source outside the passport", "create_report", `{"template":"internal_investigation_v1","source_invoice_ids":["invoice_A01","invoice_B01"]}`, nil, OutcomeDeny, ReasonResourceOutOfScope, true},
		{"template not in passport", "create_report", `{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_A01"]}`, func(scope *PassportScope) { scope.AllowedTemplates = []string{TemplateInternalInvestigation} }, OutcomeDeny, ReasonTemplateNotAllowed, true},
		{"recipient not in passport", "queue_report", `{"report_id":"eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee","recipient_reference":"recipient:other"}`, nil, OutcomeDeny, ReasonDestinationNotAllowed, true},
		{"expired passport", "read_invoice", `{"invoice_id":"invoice_A01"}`, func(scope *PassportScope) { scope.ExpiresAt = time.Now().Add(-time.Second) }, OutcomeDeny, ReasonRunCancelled, true},
		{"scope of another organization", "read_invoice", `{"invoice_id":"invoice_A01"}`, func(scope *PassportScope) { scope.OrganizationID = "ffffffff-ffff-4fff-8fff-ffffffffffff" }, OutcomeDeny, ReasonDecisionUnavailable, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			scope := atlasScope()
			if testCase.adjustScope != nil {
				testCase.adjustScope(&scope)
			}
			gate, _, recorder := newTestGate(scope)
			decision := gate.Evaluate(context.Background(), testRun(), proposal(testCase.tool, testCase.arguments))
			if decision.Outcome != testCase.wantOutcome || decision.ReasonCode != testCase.wantReason {
				t.Fatalf("decision = %s/%s, want %s/%s", decision.Outcome, decision.ReasonCode, testCase.wantOutcome, testCase.wantReason)
			}
			if decision.ActionStored != testCase.wantStored || (recorder.storedAction != nil) != testCase.wantStored {
				t.Fatalf("action stored = %v, want %v", decision.ActionStored, testCase.wantStored)
			}
			if len(recorder.decisions) != 1 {
				t.Fatalf("recorded %d decisions, want exactly 1", len(recorder.decisions))
			}
			if testCase.wantStored && (len(recorder.calls) != 2 || recorder.calls[0] != "store" || recorder.calls[1] != "decision") {
				t.Fatalf("call order = %v, want the action stored before the decision", recorder.calls)
			}
		})
	}
}

func TestStoredActionCarriesTheCanonicalFormAndDigest(t *testing.T) {
	gate, _, recorder := newTestGate(atlasScope())
	decision := gate.Evaluate(context.Background(), testRun(), proposal("read_invoice", ` { "invoice_id" : "invoice_A01" } `))
	stored := recorder.storedAction
	if stored == nil || string(stored.CanonicalArguments) != `{"invoice_id":"invoice_A01"}` {
		t.Fatalf("stored canonical arguments = %v", stored)
	}
	if stored.ActionDigest != decision.ActionDigest || stored.EvaluatedRevisionID != 3 || stored.CanonicalizationVersion != CanonicalizationVersion {
		t.Fatal("the stored action and the decision disagree on digest or revision")
	}
}

func TestGateFailsClosed(t *testing.T) {
	permitted := proposal("read_invoice", `{"invoice_id":"invoice_A01"}`)
	cases := map[string]func(*fakeScopes, *fakeRecorder){
		"scope lookup fails":    func(scopes *fakeScopes, _ *fakeRecorder) { scopes.scopeError = errors.New("database down") },
		"no active revision":    func(scopes *fakeScopes, _ *fakeRecorder) { scopes.revision = 0 },
		"revision lookup fails": func(scopes *fakeScopes, _ *fakeRecorder) { scopes.revisionErr = errors.New("database down") },
		"action store fails":    func(_ *fakeScopes, recorder *fakeRecorder) { recorder.storeError = errors.New("database down") },
		"decision record fails": func(_ *fakeScopes, recorder *fakeRecorder) { recorder.recordError = errors.New("database down") },
	}
	for name, breakSomething := range cases {
		t.Run(name, func(t *testing.T) {
			gate, scopes, recorder := newTestGate(atlasScope())
			breakSomething(scopes, recorder)
			decision := gate.Evaluate(context.Background(), testRun(), permitted)
			if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonDecisionUnavailable {
				t.Fatalf("decision = %s/%s, want deny/decision_unavailable", decision.Outcome, decision.ReasonCode)
			}
		})
	}
}

func TestInvalidProposalEnvelopeIsDenied(t *testing.T) {
	cases := map[string]func(*Proposal){
		"action id not a UUID": func(p *Proposal) { p.ActionID = "action-1" },
		"step number zero":     func(p *Proposal) { p.StepNumber = 0 },
		"no idempotency key":   func(p *Proposal) { p.IdempotencyKey = "" },
	}
	for name, breakProposal := range cases {
		t.Run(name, func(t *testing.T) {
			gate, _, recorder := newTestGate(atlasScope())
			brokenProposal := proposal("read_invoice", `{"invoice_id":"invoice_A01"}`)
			breakProposal(&brokenProposal)
			decision := gate.Evaluate(context.Background(), testRun(), brokenProposal)
			if decision.Outcome != OutcomeDeny || recorder.storedAction != nil {
				t.Fatalf("decision = %s, stored = %v; want deny and nothing stored", decision.Outcome, recorder.storedAction != nil)
			}
		})
	}
}

func TestSemanticCheckRestrictsButNeverGrants(t *testing.T) {
	read := proposal("read_invoice", `{"invoice_id":"invoice_A01"}`)
	queue := proposal("queue_report", `{"report_id":"eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee","recipient_reference":"`+testRecipient+`"}`)
	outOfScope := proposal("read_invoice", `{"invoice_id":"invoice_B01"}`)
	cases := []struct {
		name        string
		proposal    Proposal
		evaluator   *fakeEvaluator
		wantOutcome Outcome
		wantReason  ReasonCode
		wantCalls   int
	}{
		{"allow stays allow", read, &fakeEvaluator{outcome: OutcomeAllow}, OutcomeAllow, "", 1},
		{"semantic deny blocks an allowed read", read, &fakeEvaluator{outcome: OutcomeDeny, reason: "semantic_injection_detected"}, OutcomeDeny, "semantic_injection_detected", 1},
		{"semantic allow never skips review", queue, &fakeEvaluator{outcome: OutcomeAllow}, OutcomeApprovalRequired, ReasonApprovalRequired, 1},
		{"evaluator error denies", read, &fakeEvaluator{err: errors.New("timeout")}, OutcomeDeny, ReasonDecisionUnavailable, 1},
		{"unknown verdict denies", read, &fakeEvaluator{outcome: "maybe"}, OutcomeDeny, ReasonDecisionUnavailable, 1},
		{"deterministic denial needs no semantic call", outOfScope, &fakeEvaluator{outcome: OutcomeAllow}, OutcomeDeny, ReasonResourceOutOfScope, 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			gate := NewGate(&fakeScopes{scope: atlasScope(), revision: 3}, &fakeRecorder{}, testCase.evaluator)
			decision := gate.Evaluate(context.Background(), testRun(), testCase.proposal)
			if decision.Outcome != testCase.wantOutcome || decision.ReasonCode != testCase.wantReason {
				t.Fatalf("decision = %s/%s, want %s/%s", decision.Outcome, decision.ReasonCode, testCase.wantOutcome, testCase.wantReason)
			}
			if testCase.evaluator.calls != testCase.wantCalls {
				t.Fatalf("evaluator calls = %d, want %d", testCase.evaluator.calls, testCase.wantCalls)
			}
		})
	}
}
