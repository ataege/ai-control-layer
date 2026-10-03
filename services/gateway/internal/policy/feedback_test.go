package policy

import (
	"encoding/json"
	"strings"
	"testing"

	"starter/services/gateway/internal/contracts"
)

func TestDenialFeedbackOffersOnlyPermittedAlternatives(t *testing.T) {
	exportDenial := Decision{Outcome: OutcomeDeny, ReasonCode: ReasonReportExportRestricted, AlternativeTemplate: TemplateVendorReconciliation}
	cases := []struct {
		name            string
		adjustScope     func(*PassportScope)
		wantAlternative string
	}{
		{"passport permits the vendor template", nil, TemplateVendorReconciliation},
		{"vendor template not in the passport", func(scope *PassportScope) { scope.AllowedTemplates = []string{TemplateInternalInvestigation} }, ""},
		{"create_report not in the passport", func(scope *PassportScope) { scope.AllowedTools = []ToolName{ToolQueueReport} }, ""},
		{"no invoice sources", func(scope *PassportScope) { scope.AllowedInvoiceIDs = nil }, ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			scope := atlasScope()
			if testCase.adjustScope != nil {
				testCase.adjustScope(&scope)
			}
			feedback := BuildDenialFeedback(exportDenial, scope)
			if feedback.AlternativeTemplate != testCase.wantAlternative || feedback.ReasonCode != ReasonReportExportRestricted {
				t.Fatalf("feedback = %+v, want alternative %q", feedback, testCase.wantAlternative)
			}
		})
	}
}

// Every X-13 reason code yields a specific message, from this package's feedback texts or from the
// contracts table, never the generic fallback (a multi-action denial once told the model only that).
func TestEveryReasonCodeHasASpecificFeedbackMessage(t *testing.T) {
	scope := atlasScope()
	for _, code := range contracts.ReasonCodes {
		feedback := BuildDenialFeedback(Decision{Outcome: OutcomeDeny, ReasonCode: ReasonCode(code)}, scope)
		if feedback.SafeMessage == "" || feedback.SafeMessage == genericSafeMessage || feedback.SafeMessage == contracts.ReasonCode("").SafeMessage() {
			t.Errorf("%s has no specific feedback message (got %q)", code, feedback.SafeMessage)
		}
	}
	multiple := BuildDenialFeedback(Decision{Outcome: OutcomeDeny, ReasonCode: ReasonCode(contracts.ReasonMultipleActionsNotSupported)}, scope)
	if !strings.HasPrefix(multiple.SafeMessage, contracts.ReasonMultipleActionsNotSupported.SafeMessage()) {
		t.Fatalf("multiple actions feedback = %q", multiple.SafeMessage)
	}
}

func TestDenialFeedbackHoldsNoProtectedValue(t *testing.T) {
	scope := atlasScope()
	for _, code := range contracts.ReasonCodes {
		reason := ReasonCode(code)
		feedback := BuildDenialFeedback(Decision{Outcome: OutcomeDeny, ReasonCode: reason}, scope)
		encoded, _ := json.Marshal(feedback)
		for _, protected := range []string{"@", "Investigation note", testRecipient, "INV104"} {
			if strings.Contains(string(encoded), protected) {
				t.Fatalf("feedback for %s contains %q: %s", reason, protected, encoded)
			}
		}
	}
	unknown := BuildDenialFeedback(Decision{Outcome: OutcomeDeny, ReasonCode: "something_new"}, scope)
	if unknown.SafeMessage != genericSafeMessage {
		t.Fatalf("an unknown reason got %q", unknown.SafeMessage)
	}
}

func TestOutOfScopeFeedbackNamesThePermittedRecords(t *testing.T) {
	feedback := BuildDenialFeedback(Decision{Outcome: OutcomeDeny, ReasonCode: ReasonResourceOutOfScope}, atlasScope())
	if len(feedback.PermittedInvoiceIDs) != 2 || feedback.PermittedInvoiceIDs[0] != "invoice_A01" {
		t.Fatalf("permitted invoices = %v", feedback.PermittedInvoiceIDs)
	}
}

func TestCorrectionLimit(t *testing.T) {
	cases := []struct {
		used, limit  int
		wantContinue bool
	}{
		{1, 2, true}, {2, 2, true}, {3, 2, false}, {1, 0, false}, {0, 0, true}, {1, -1, false},
	}
	for _, testCase := range cases {
		verdict := CheckCorrections(testCase.used, testCase.limit)
		if verdict.Continue != testCase.wantContinue {
			t.Fatalf("used %d of %d: continue = %v, want %v", testCase.used, testCase.limit, verdict.Continue, testCase.wantContinue)
		}
		if !verdict.Continue && verdict.StopReason != ReasonAllowanceExhausted {
			t.Fatalf("stop reason = %q", verdict.StopReason)
		}
	}
}

// Denials that can never succeed on retry tell the model not to propose the action again;
// denials a corrected proposal can pass do not.
func TestFinalDenialsSayDoNotProposeAgain(t *testing.T) {
	scope := atlasScope()
	for reason := range finalDenials {
		feedback := BuildDenialFeedback(Decision{Outcome: OutcomeDeny, ReasonCode: reason}, scope)
		if !strings.HasSuffix(feedback.SafeMessage, " "+doNotRetrySentence) {
			t.Errorf("%s: %q lacks the do-not-retry sentence", reason, feedback.SafeMessage)
		}
	}
	for _, reason := range []ReasonCode{ReasonInvalidArguments, ReasonActionChanged, contracts.ReasonResourceVersionChanged,
		contracts.ReasonApprovalExpired, ReasonDecisionUnavailable} {
		feedback := BuildDenialFeedback(Decision{Outcome: OutcomeDeny, ReasonCode: reason}, scope)
		if strings.Contains(feedback.SafeMessage, doNotRetrySentence) {
			t.Errorf("%s is retryable but says %q", reason, feedback.SafeMessage)
		}
	}
	// The permitted alternative stays when the passport permits it.
	export := BuildDenialFeedback(Decision{Outcome: OutcomeDeny, ReasonCode: ReasonReportExportRestricted,
		AlternativeTemplate: TemplateVendorReconciliation}, scope)
	if export.AlternativeTemplate != TemplateVendorReconciliation || !strings.Contains(export.SafeMessage, doNotRetrySentence) {
		t.Fatalf("export denial feedback = %+v", export)
	}
}

// Once the vendor report is queued, a denied internal export no longer offers the alternative and
// tells the model to finish; without that progress, or for another denial, nothing changes.
func TestCorrectionFeedbackSaysWhenTheAlternativeIsDone(t *testing.T) {
	scope := atlasScope()
	export := Decision{Outcome: OutcomeDeny, ReasonCode: ReasonReportExportRestricted, AlternativeTemplate: TemplateVendorReconciliation}
	done := BuildCorrectionFeedback(export, scope, RunProgress{VendorReportQueued: true})
	if done.AlternativeTemplate != "" || !strings.HasSuffix(done.SafeMessage, " "+alternativeCompletedSentence) ||
		!strings.Contains(done.SafeMessage, doNotRetrySentence) {
		t.Fatalf("with the vendor report queued: %+v", done)
	}
	pending := BuildCorrectionFeedback(export, scope, RunProgress{})
	if pending.AlternativeTemplate != TemplateVendorReconciliation || strings.Contains(pending.SafeMessage, alternativeCompletedSentence) {
		t.Fatalf("without progress: %+v", pending)
	}
	other := BuildCorrectionFeedback(Decision{Outcome: OutcomeDeny, ReasonCode: ReasonResourceOutOfScope}, scope, RunProgress{VendorReportQueued: true})
	if strings.Contains(other.SafeMessage, alternativeCompletedSentence) {
		t.Fatalf("an out-of-scope denial says the alternative is done: %+v", other)
	}
}
