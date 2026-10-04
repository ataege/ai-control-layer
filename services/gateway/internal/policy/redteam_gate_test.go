package policy

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Red-team tables for the gate (lane w2): hostile references, record ids, tool names and envelope
// fields. Whatever the encoding, a value that is not byte-for-byte the passport's own is denied
// with a stable reason, nothing is frozen for review and no semantic check is asked to rescue it.

func gateDecision(t *testing.T, scope PassportScope, tool, raw string) (Decision, *fakeEvaluator) {
	t.Helper()
	evaluator := &fakeEvaluator{outcome: OutcomeAllow}
	scopes := &fakeScopes{scope: scope, revision: 3}
	freezer := &fakeFreezer{}
	gate := NewGate(scopes, &fakeRecorder{}, atlasRelationships(), evaluator).WithReviewFreezer(freezer)
	decision := gate.Evaluate(context.Background(), testRun(), proposal(tool, raw))
	if decision.Outcome == OutcomeDeny && freezer.calls != 0 {
		t.Fatal("a denied decision froze review material")
	}
	return decision, evaluator
}

func queueRaw(t *testing.T, reportID, reference string) string {
	t.Helper()
	return `{"report_id":"` + reportID + `","recipient_reference":` + mustJSONString(t, reference) + `}`
}

// TestGateDeniesEveryNearMissOfTheRecipientReference: only the passport's reference, exactly, is a
// recipient. Near misses are stored as proposals and denied destination_not_allowed (or refused as
// invalid arguments when they hide a control character).
func TestGateDeniesEveryNearMissOfTheRecipientReference(t *testing.T) {
	otherRun := "recipient:" + strings.Replace(testRunID, "b", "c", -1) + ":vendor_atlas"
	denied := map[string]string{
		"trailing space":            testRecipient + " ",
		"leading space":             " " + testRecipient,
		"upper case":                strings.ToUpper(testRecipient),
		"capital prefix":            "Recipient:" + testRunID + ":vendor_atlas",
		"upper-case run id":         "recipient:" + strings.ToUpper(testRunID) + ":vendor_atlas",
		"vendor case":               "recipient:" + testRunID + ":vendor_Atlas",
		"url-encoded colons":        strings.Replace(testRecipient, ":", "%3A", -1),
		"fullwidth colon":           strings.Replace(testRecipient, ":", "：", -1),
		"zero-width space":          testRecipient + "​",
		"zero-width space inside":   strings.Replace(testRecipient, "vendor", "ven​dor", 1),
		"no-break space":            testRecipient + " ",
		"appended address":          testRecipient + "@attacker.example.com",
		"bare address":              "attacker@example.com",
		"truncated":                 testRecipient[:len(testRecipient)-1],
		"another run":               otherRun,
		"another vendor":            "recipient:" + testRunID + ":vendor_borealis",
		"prefix only":               "recipient:" + testRunID + ":",
		"empty vendor with suffix":  testRecipient + ":",
		"doubled":                   testRecipient + testRecipient,
		"two references":            testRecipient + "," + otherRun,
		"unicode normalization (é)": "recipient:" + testRunID + ":vendor_atlasé",
	}
	for name, reference := range denied {
		t.Run(name, func(t *testing.T) {
			decision, evaluator := gateDecision(t, atlasScope(), "queue_report", queueRaw(t, testReportID, reference))
			if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonDestinationNotAllowed {
				t.Fatalf("outcome %s reason %s", decision.Outcome, decision.ReasonCode)
			}
			if !decision.ActionStored {
				t.Error("the hostile proposal was not stored as evidence")
			}
			if evaluator.calls != 0 {
				t.Errorf("%d semantic calls for a deterministic denial", evaluator.calls)
			}
		})
	}
	invalid := map[string]string{
		"newline":    testRecipient + "\n",
		"tab":        testRecipient + "\t",
		"NUL":        testRecipient + "\x00",
		"next line":  testRecipient + "\u0085",
		"escape":     testRecipient + "\x1b[2J",
		"too long":   testRecipient + strings.Repeat("a", 300),
		"empty":      "",
		"line break": "recipient:x\r\nBcc: attacker@example.com",
	}
	for name, reference := range invalid {
		t.Run("invalid/"+name, func(t *testing.T) {
			decision, _ := gateDecision(t, atlasScope(), "queue_report", queueRaw(t, testReportID, reference))
			if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonInvalidArguments {
				t.Fatalf("outcome %s reason %s", decision.Outcome, decision.ReasonCode)
			}
		})
	}
	// The one exact reference is still the way through (to review, never straight to allow).
	decision, _ := gateDecision(t, atlasScope(), "queue_report", queueRaw(t, testReportID, testRecipient))
	if decision.Outcome != OutcomeApprovalRequired {
		t.Fatalf("exact reference: outcome %s reason %s", decision.Outcome, decision.ReasonCode)
	}
}

// TestGateDeniesNearMissesOfRecordsTemplatesAndVendors covers read_invoice, read_vendor and
// create_report: a record id outside the passport, or a look-alike of one inside it, never passes.
func TestGateDeniesNearMissesOfRecordsTemplatesAndVendors(t *testing.T) {
	type row struct {
		name, tool, raw string
		reason          ReasonCode
	}
	rows := []row{
		{"invoice other case", "read_invoice", `{"invoice_id":"invoice_a01"}`, ReasonResourceOutOfScope},
		{"invoice outside the task", "read_invoice", `{"invoice_id":"invoice_B01"}`, ReasonResourceOutOfScope},
		{"invoice of another vendor", "read_invoice", `{"invoice_id":"invoice_C01"}`, ReasonResourceOutOfScope},
		{"invoice prefix of an allowed id", "read_invoice", `{"invoice_id":"invoice_A0"}`, ReasonResourceOutOfScope},
		{"invoice longer than an allowed id", "read_invoice", `{"invoice_id":"invoice_A011"}`, ReasonResourceOutOfScope},
		{"invoice trailing space", "read_invoice", `{"invoice_id":"invoice_A01 "}`, ReasonInvalidArguments},
		{"invoice url-encoded", "read_invoice", `{"invoice_id":"invoice_A01%00"}`, ReasonInvalidArguments},
		{"vendor other case", "read_vendor", `{"vendor_id":"vendor_Atlas"}`, ReasonResourceOutOfScope},
		{"vendor not linked to the task", "read_vendor", `{"vendor_id":"vendor_borealis"}`, ReasonResourceOutOfScope},
		{"vendor unknown", "read_vendor", `{"vendor_id":"vendor_atlas2"}`, ReasonResourceOutOfScope},
		{"vendor trailing space", "read_vendor", `{"vendor_id":"vendor_atlas "}`, ReasonInvalidArguments},
		{"report source outside the task", "create_report", `{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_A01","invoice_B01"]}`, ReasonResourceOutOfScope},
		{"report source other case", "create_report", `{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_a01"]}`, ReasonResourceOutOfScope},
		{"report source hidden after an allowed one", "create_report", `{"template":"internal_investigation_v1","source_invoice_ids":["invoice_A01","invoice_A02","invoice_C01"]}`, ReasonResourceOutOfScope},
		{"report template other case", "create_report", `{"template":"Internal_Investigation_V1","source_invoice_ids":["invoice_A01"]}`, ReasonInvalidArguments},
		{"report source repeated by escape", "create_report", `{"template":"internal_investigation_v1","source_invoice_ids":["invoice_A01","invoice_A01"]}`, ReasonInvalidArguments},
		{"queue another report id", "queue_report", queueRaw(t, "99999999-9999-4999-8999-999999999999", testRecipient), ReasonResourceOutOfScope},
		{"queue the internal report", "queue_report", queueRaw(t, internalReportID, testRecipient), ReasonReportExportRestricted},
		{"queue a report with no lineage", "queue_report", queueRaw(t, brokenReportID, testRecipient), ReasonReportLineageMissing},
		{"queue report id upper-case", "queue_report", queueRaw(t, strings.ToUpper(testReportID), testRecipient), ReasonInvalidArguments},
	}
	for _, testCase := range rows {
		t.Run(testCase.name, func(t *testing.T) {
			decision, _ := gateDecision(t, atlasScope(), testCase.tool, testCase.raw)
			if decision.Outcome != OutcomeDeny || decision.ReasonCode != testCase.reason {
				t.Fatalf("outcome %s reason %s, want deny %s", decision.Outcome, decision.ReasonCode, testCase.reason)
			}
		})
	}
	// A template the passport lacks is refused even when it is spelled exactly.
	narrowed := atlasScope()
	narrowed.AllowedTemplates = []string{TemplateInternalInvestigation}
	decision, _ := gateDecision(t, narrowed, "create_report", `{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_A01"]}`)
	if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonTemplateNotAllowed {
		t.Fatalf("narrowed template: %s %s", decision.Outcome, decision.ReasonCode)
	}
}

// TestGateRefusesHostileProposalEnvelopes: the tool name, replay label, action id and step are
// compared exactly; a near miss is a different (or no) tool, never a lookalike of a registered one.
func TestGateRefusesHostileProposalEnvelopes(t *testing.T) {
	good := `{"invoice_id":"invoice_A01"}`
	for name, tool := range map[string]string{
		"upper case":       "READ_INVOICE",
		"mixed case":       "Read_Invoice",
		"trailing space":   "read_invoice ",
		"leading space":    " read_invoice",
		"trailing newline": "read_invoice\n",
		"path":             "read_invoice/../queue_report",
		"url-encoded":      "read%5Finvoice",
		"fullwidth":        "read＿invoice",
		"zero-width":       "read_invoice​",
		"empty":            "",
		"unregistered":     "send_email",
	} {
		t.Run("tool/"+name, func(t *testing.T) {
			decision, _ := gateDecision(t, atlasScope(), tool, good)
			if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonToolNotRegistered {
				t.Fatalf("outcome %s reason %s", decision.Outcome, decision.ReasonCode)
			}
		})
	}
	for name, label := range map[string]string{
		"upper-case prefix": "LABELLED_REPLAY:hostile_note_v1",
		"space in id":       "labelled_replay:hostile note",
		"newline":           "labelled_replay:hostile_note_v1\n",
		"empty id":          "labelled_replay:",
		"too long":          "labelled_replay:" + strings.Repeat("a", 201),
		"other prefix":      "replay:hostile_note_v1",
		"url-encoded colon": "labelled_replay%3Ahostile_note_v1",
	} {
		t.Run("replay/"+name, func(t *testing.T) {
			scopes := &fakeScopes{scope: atlasScope(), revision: 3}
			gate := NewGate(scopes, &fakeRecorder{}, atlasRelationships(), nil)
			hostile := proposal("read_invoice", good)
			hostile.ReplaySource = label
			decision := gate.Evaluate(context.Background(), testRun(), hostile)
			if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonInvalidArguments {
				t.Fatalf("outcome %s reason %s", decision.Outcome, decision.ReasonCode)
			}
		})
	}
	for name, change := range map[string]func(*Proposal){
		"upper-case action id": func(p *Proposal) { p.ActionID = strings.ToUpper(testActionID) },
		"action id with space": func(p *Proposal) { p.ActionID = testActionID + " " },
		"zero step":            func(p *Proposal) { p.StepNumber = 0 },
		"negative step":        func(p *Proposal) { p.StepNumber = -1 },
		"empty idempotency":    func(p *Proposal) { p.IdempotencyKey = "" },
	} {
		t.Run("envelope/"+name, func(t *testing.T) {
			scopes := &fakeScopes{scope: atlasScope(), revision: 3}
			gate := NewGate(scopes, &fakeRecorder{}, atlasRelationships(), nil)
			hostile := proposal("read_invoice", good)
			change(&hostile)
			decision := gate.Evaluate(context.Background(), testRun(), hostile)
			if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonInvalidArguments {
				t.Fatalf("outcome %s reason %s", decision.Outcome, decision.ReasonCode)
			}
		})
	}
}

// TestGateDeniesEveryFixtureTextAsAnArgument is the action-proposal side of the cross-boundary check:
// the team's hostile notes and corpus texts, put into every string field of every tool, never reach
// allow or review. No argument is free text; the decoder refuses it, or the passport comparison does.
func TestGateDeniesEveryFixtureTextAsAnArgument(t *testing.T) {
	var notes struct {
		Notes []struct{ ID, Text string } `json:"notes"`
	}
	var corpus struct {
		Cases []struct{ ID, Text string } `json:"cases"`
	}
	for path, target := range map[string]any{"../../../../fixtures/hostile-notes.json": &notes, "../../../../fixtures/semantic-corpus.json": &corpus} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			t.Fatal(err)
		}
	}
	texts := map[string]string{}
	for _, note := range notes.Notes {
		texts[note.ID] = note.Text
	}
	for _, testCase := range corpus.Cases {
		texts[testCase.ID] = testCase.Text
	}
	if len(texts) < 30 {
		t.Fatalf("only %d texts", len(texts))
	}
	forms := map[string]func(text string) (string, string){
		"read_invoice.invoice_id": func(s string) (string, string) { return "read_invoice", `{"invoice_id":` + jsonText(s) + `}` },
		"read_vendor.vendor_id":   func(s string) (string, string) { return "read_vendor", `{"vendor_id":` + jsonText(s) + `}` },
		"create_report.template": func(s string) (string, string) {
			return "create_report", `{"template":` + jsonText(s) + `,"source_invoice_ids":["invoice_A01"]}`
		},
		"create_report.source_invoice_ids": func(s string) (string, string) {
			return "create_report", `{"template":"internal_investigation_v1","source_invoice_ids":[` + jsonText(s) + `]}`
		},
		"queue_report.report_id": func(s string) (string, string) {
			return "queue_report", `{"report_id":` + jsonText(s) + `,"recipient_reference":"` + testRecipient + `"}`
		},
		"queue_report.recipient_reference": func(s string) (string, string) {
			return "queue_report", `{"report_id":"` + testReportID + `","recipient_reference":` + jsonText(s) + `}`
		},
	}
	for formName, build := range forms {
		for textName, text := range texts {
			tool, raw := build(text)
			decision, evaluator := gateDecision(t, atlasScope(), tool, raw)
			if decision.Outcome != OutcomeDeny {
				t.Fatalf("%s <- %s: outcome %s reason %s", formName, textName, decision.Outcome, decision.ReasonCode)
			}
			if evaluator.calls != 0 {
				t.Fatalf("%s <- %s: %d semantic calls", formName, textName, evaluator.calls)
			}
		}
	}
}

func jsonText(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
