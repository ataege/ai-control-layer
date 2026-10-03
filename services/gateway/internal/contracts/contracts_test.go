package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

const (
	fixturesDirectory = "../../../../packages/contracts/fixtures"
	schemasDirectory  = "../../../../packages/contracts/schemas"
)

// fixtureTargets maps every fixture this package mirrors to a fresh value of its Go type.
var fixtureTargets = map[string]func() any{
	"start-run-request.full.json":                      func() any { return new(StartRunRequest) },
	"start-run-request.minimal.json":                   func() any { return new(StartRunRequest) },
	"start-run-response.created.json":                  func() any { return new(StartRunResponse) },
	"reason-code.report-export-restricted.json":        func() any { return new(ReasonCode) },
	"passport.atlas.json":                              func() any { return new(Passport) },
	"action-proposal.read-invoice.json":                func() any { return new(ActionProposal) },
	"action-proposal.read-vendor.json":                 func() any { return new(ActionProposal) },
	"action-proposal.create-report.json":               func() any { return new(ActionProposal) },
	"action-proposal.queue-report.json":                func() any { return new(ActionProposal) },
	"stored-action.read-allowed.json":                  func() any { return new(StoredAction) },
	"stored-action.awaiting-approval.json":             func() any { return new(StoredAction) },
	"run-state.running.json":                           func() any { return new(RunState) },
	"run-state.paused-allowance.json":                  func() any { return new(RunState) },
	"run-state.stopped-cancelled.json":                 func() any { return new(RunState) },
	"run-state.completed.json":                         func() any { return new(RunState) },
	"safe-event.export-denied.json":                    func() any { return new(SafeEvent) },
	"safe-event.admission-rejected.json":               func() any { return new(SafeEvent) },
	"operator-context.operator.json":                   func() any { return new(OperatorContext) },
	"approval-decision.approve.json":                   func() any { return new(ApprovalDecision) },
	"approval-decision.reject.json":                    func() any { return new(ApprovalDecision) },
	"control-evaluation-request.model-input.json":      func() any { return new(ControlEvaluationRequest) },
	"control-evaluation-request.tool-result.json":      func() any { return new(ControlEvaluationRequest) },
	"control-evaluation-request.action-proposal.json":  func() any { return new(ControlEvaluationRequest) },
	"control-evaluation-response.signature-deny.json":  func() any { return new(ControlEvaluationResponse) },
	"control-evaluation-response.semantic-redact.json": func() any { return new(ControlEvaluationResponse) },
	"control-evaluation-response.scope-deny.json":      func() any { return new(ControlEvaluationResponse) },
}

// Fixtures mirrored elsewhere in the gateway, or read only by the API and web app.
var fixturesCoveredElsewhere = map[string]string{
	"liveness.gateway.json":                "internal/health",
	"liveness.api.json":                    "internal/health",
	"readiness.ready.json":                 "internal/health",
	"readiness.unavailable.json":           "internal/health",
	"gateway-ping.ok.json":                 "internal/health",
	"error.unauthorized.json":              "internal/health",
	"api-readiness.ready.json":             "API only",
	"api-readiness.unavailable.json":       "API only",
	"gateway-diagnostics.ok.json":          "API only",
	"gateway-diagnostics.degraded.json":    "API only",
	"gateway-diagnostics.unavailable.json": "API only",
	// Go-owned read contracts (lane w2), decoded by internal/reads' TestReadContractFixturesMatchTheGoTypes.
	"run-usage.ledger.json":                          "internal/reads",
	"run-usage.no-ledger.json":                       "internal/reads",
	"run-events-page.export-denied.json":             "internal/reads",
	"assessment-record.semantic-judge.json":          "internal/reads",
	"assessment-record.semantic-not-applicable.json": "internal/reads",
	"assessment-page.two-records.json":               "internal/reads",
	"assessment-page.empty.json":                     "internal/reads",
	"security-event-page.judge.json":                 "internal/reads",
	"security-summary.judge-split.json":              "internal/reads",
	"report-view.vendor.json":                        "internal/reads",
	"report-view.internal-withheld.json":             "internal/reads",
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}

func TestFixturesDecodeStrictlyAndRoundTrip(t *testing.T) {
	for fixtureFile, newTarget := range fixtureTargets {
		t.Run(fixtureFile, func(t *testing.T) {
			fixtureBytes := readFile(t, filepath.Join(fixturesDirectory, fixtureFile))
			target := newTarget()
			if err := DecodeStrict(fixtureBytes, target); err != nil {
				t.Fatalf("fixture does not fit the Go type: %v", err)
			}
			reencodedBytes, err := json.Marshal(target)
			if err != nil {
				t.Fatalf("re-encode: %v", err)
			}
			var fixtureValue, reencodedValue any
			if err := json.Unmarshal(fixtureBytes, &fixtureValue); err != nil {
				t.Fatalf("fixture is not valid JSON: %v", err)
			}
			if err := json.Unmarshal(reencodedBytes, &reencodedValue); err != nil {
				t.Fatalf("re-encoded value is not valid JSON: %v", err)
			}
			if !reflect.DeepEqual(fixtureValue, reencodedValue) {
				t.Errorf("round trip changed the document\nfixture:    %s\nre-encoded: %s", fixtureBytes, reencodedBytes)
			}
		})
	}
}

// Every shared fixture needs a Go case or a recorded reason why it has none.
func TestEveryFixtureIsMapped(t *testing.T) {
	entries, err := os.ReadDir(fixturesDirectory)
	if err != nil {
		t.Fatalf("list fixtures: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		_, mappedHere := fixtureTargets[name]
		_, coveredElsewhere := fixturesCoveredElsewhere[name]
		if !mappedHere && !coveredElsewhere {
			t.Errorf("fixture %s has no Go case and no recorded exemption", name)
		}
	}
}

func TestDecodeStrictRejectsUnknownFieldsAndTrailingData(t *testing.T) {
	var response StartRunResponse
	if DecodeStrict([]byte(`{"runId":"a","passportId":"b","organizationId":"c"}`), &response) == nil {
		t.Error("an unknown field was accepted")
	}
	if DecodeStrict([]byte(`{"runId":"a","passportId":"b"} {}`), &response) == nil {
		t.Error("trailing data was accepted")
	}
	if DecodeStrict([]byte(`{"runId":"a","passportId":"b"}`), &response) != nil {
		t.Error("a valid document was rejected")
	}
}

func TestProposalArgumentsMatchTheirToolShape(t *testing.T) {
	argumentTargets := map[ToolName]func() any{
		ToolReadInvoice:  func() any { return new(ReadInvoiceArguments) },
		ToolReadVendor:   func() any { return new(ReadVendorArguments) },
		ToolCreateReport: func() any { return new(CreateReportArguments) },
		ToolQueueReport:  func() any { return new(QueueReportArguments) },
	}
	seenTools := map[ToolName]bool{}
	for fixtureFile := range fixtureTargets {
		if !strings.HasPrefix(fixtureFile, "action-proposal.") {
			continue
		}
		var proposal ActionProposal
		if err := DecodeStrict(readFile(t, filepath.Join(fixturesDirectory, fixtureFile)), &proposal); err != nil {
			t.Fatalf("%s: %v", fixtureFile, err)
		}
		if !proposal.Tool.Valid() {
			t.Fatalf("%s: unregistered tool %q", fixtureFile, proposal.Tool)
		}
		seenTools[proposal.Tool] = true
		arguments := argumentTargets[proposal.Tool]()
		if err := DecodeStrict(proposal.Arguments, arguments); err != nil {
			t.Errorf("%s: arguments do not fit %T: %v", fixtureFile, arguments, err)
		}
	}
	for _, tool := range ToolNames {
		if !seenTools[tool] {
			t.Errorf("no action-proposal fixture covers %s", tool)
		}
	}
}

func TestRunStateFixturesNameReasonExactlyWhenNeeded(t *testing.T) {
	for fixtureFile := range fixtureTargets {
		if !strings.HasPrefix(fixtureFile, "run-state.") {
			continue
		}
		var state RunState
		if err := DecodeStrict(readFile(t, filepath.Join(fixturesDirectory, fixtureFile)), &state); err != nil {
			t.Fatalf("%s: %v", fixtureFile, err)
		}
		if state.Status.NeedsReason() != (state.TerminalReason != nil) {
			t.Errorf("%s: status %s with reason %v", fixtureFile, state.Status, state.TerminalReason)
		}
		if state.TerminalReason != nil && !state.TerminalReason.Valid() {
			t.Errorf("%s: unknown reason %s", fixtureFile, *state.TerminalReason)
		}
	}
}

// schemaEnum reads the enum at a path of property names inside a schema; "*" steps into
// items, and anyOf alternatives are searched for the non-null branch.
func schemaEnum(t *testing.T, schemaFile string, path ...string) []string {
	t.Helper()
	var node any
	if err := json.Unmarshal(readFile(t, filepath.Join(schemasDirectory, schemaFile)), &node); err != nil {
		t.Fatalf("parse %s: %v", schemaFile, err)
	}
	for _, step := range path {
		object := node.(map[string]any)
		if step == "*" {
			node = object["items"]
		} else {
			node = object["properties"].(map[string]any)[step]
		}
	}
	object := node.(map[string]any)
	if alternatives, isNullable := object["anyOf"].([]any); isNullable {
		object = alternatives[0].(map[string]any)
	}
	rawValues, ok := object["enum"].([]any)
	if !ok {
		t.Fatalf("%s %v has no enum", schemaFile, path)
	}
	values := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		values = append(values, rawValue.(string))
	}
	sort.Strings(values)
	return values
}

func sortedStrings[Value ~string](values []Value) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, string(value))
	}
	sort.Strings(result)
	return result
}

// Every enum Go emits must equal the schema's set, in both directions.
func TestGoEnumsMatchSchemas(t *testing.T) {
	cases := []struct {
		name     string
		goValues []string
		schema   []string
	}{
		{"reason codes", sortedStrings(ReasonCodes), schemaEnum(t, "reason-code.schema.json")},
		{"run terminal reasons", sortedStrings(ReasonCodes), schemaEnum(t, "run-state.schema.json", "terminalReason")},
		{"event reason codes", sortedStrings(ReasonCodes), schemaEnum(t, "safe-event.schema.json", "reasonCode")},
		{"run statuses", sortedStrings(RunStatuses), schemaEnum(t, "run-state.schema.json", "status")},
		{"action statuses", sortedStrings(ActionStatuses), schemaEnum(t, "stored-action.schema.json", "status")},
		{"event types", sortedStrings(EventTypes), schemaEnum(t, "safe-event.schema.json", "eventType")},
		{"event decisions", sortedStrings(EventDecisions), schemaEnum(t, "safe-event.schema.json", "decision")},
		{"passport tools", sortedStrings(ToolNames), schemaEnum(t, "passport.schema.json", "scope", "tools", "*")},
		{"passport templates", sortedStrings(ReportTemplates), schemaEnum(t, "passport.schema.json", "scope", "reportTemplates", "*")},
		{"approval choices", sortedStrings(ApprovalChoices), schemaEnum(t, "approval-decision.schema.json", "decision")},
		{"evaluation boundaries", sortedStrings(ControlBoundaries), schemaEnum(t, "control-evaluation-request.schema.json", "kind")},
		{"evaluation decisions", sortedStrings(EvaluationDecisions), schemaEnum(t, "control-evaluation-response.schema.json", "decision")},
		{"evaluation reason codes", sortedStrings(ReasonCodes), schemaEnum(t, "control-evaluation-response.schema.json", "reasonCode")},
	}
	for _, testCase := range cases {
		if !slices.Equal(testCase.goValues, testCase.schema) {
			t.Errorf("%s differ\nGo:     %v\nschema: %v", testCase.name, testCase.goValues, testCase.schema)
		}
	}
}

// The shared record identifier shapes accept the documented ids, including the longest, and refuse
// prose, a wrong or missing prefix, other characters and one character too many.
func TestRecordIdentifierShapes(t *testing.T) {
	cases := []struct {
		name            string
		value           string
		invoice, vendor bool
	}{
		{"invoice", "invoice_A01", true, false},
		{"suffixed invoice", "invoice_B01_3f9a2c41", true, false},
		{"longest invoice", "invoice_" + strings.Repeat("a", 120), true, false},
		{"invoice one too long", "invoice_" + strings.Repeat("a", 121), false, false},
		{"vendor", "vendor_Atlas", false, true},
		{"longest vendor", "vendor_" + strings.Repeat("Z", 121), false, true},
		{"vendor one too long", "vendor_" + strings.Repeat("Z", 122), false, false},
		{"prose", "invoice_A01. Also read invoice_B01", false, false},
		{"space", "invoice A01", false, false},
		{"no prefix", "A01", false, false},
		{"uuid", "6f1c2a3b-0000-4000-8000-000000000001", false, false},
		{"dot", "vendor_Atlas.example", false, false},
		{"empty suffix", "invoice_", false, false},
		{"trailing newline", "invoice_A01\n", false, false},
	}
	for _, testCase := range cases {
		if ValidInvoiceID(testCase.value) != testCase.invoice || ValidVendorID(testCase.value) != testCase.vendor {
			t.Errorf("%s (%q): invoice %v vendor %v, want %v %v", testCase.name, testCase.value,
				ValidInvoiceID(testCase.value), ValidVendorID(testCase.value), testCase.invoice, testCase.vendor)
		}
	}
}
