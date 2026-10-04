package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/runresult"
)

// Live run fa417b6b: after the approved queue_report succeeded, the model proposed a tool that does
// not exist (`status`) and then a final answer with text around the JSON, and the run stopped at its
// corrections instead of completing. The model needs to be told, in the context it is shown, that the
// work is finished and only the final answer remains. These tests read the built context; no model.

func contextPassport() contracts.Passport {
	return contracts.Passport{
		TaskVersion: "reconcile_atlas_v1",
		Scope: contracts.PassportScope{
			VendorIDs: []string{"vendor_Atlas"}, InvoiceIDs: []string{"invoice_A01", "invoice_A02"},
			RecipientReferences: []string{"recipient:run:vendor_Atlas"},
			ReportTemplates:     []contracts.ReportTemplate{contracts.TemplateInternalInvestigation, contracts.TemplateVendorReconciliation},
		},
	}
}

func callEntryFor(step int, tool string) ContextEntry {
	return ContextEntry{StepNumber: step, Kind: entryAssistantCall, Content: json.RawMessage(`{"tool":"` + tool + `","arguments":{}}`)}
}

func resultEntryFor(step int) ContextEntry {
	return ContextEntry{StepNumber: step, Kind: entryToolResult, Content: json.RawMessage(`{"status":"queued_simulated"}`)}
}

func TestTaskTextSaysTheTaskEndsWhenTheReportIsQueued(t *testing.T) {
	messages := buildTaskContext(contextPassport(), nil)
	if len(messages) != 1 || messages[0].Role != "user" ||
		!strings.Contains(messages[0].Content, "Once the report for the vendor has been queued, the task is finished: call no further tool and give the final answer.") {
		t.Fatalf("task message: %+v", messages)
	}
}

func TestAReportQueuedResultIsFollowedByTheFinishMessage(t *testing.T) {
	entries := []ContextEntry{callEntryFor(6, "create_report"), resultEntryFor(6), callEntryFor(7, "queue_report"), resultEntryFor(7)}
	messages := buildTaskContext(contextPassport(), entries)
	last := messages[len(messages)-1]
	if last.Role != "user" || last.Content != reportQueuedMessage {
		t.Fatalf("last message %+v, want the fixed finish message as a user message", last)
	}
	if !strings.Contains(last.Content, "Do not call any more tools.") || !strings.Contains(last.Content, runresult.FinalAnswerInstruction) {
		t.Fatalf("the finish message lacks the stop or the exact final format: %q", last.Content)
	}
	// The finish message is appended, never inserted: the queue step's call and result come before it,
	// and everything before the queue step is unchanged.
	without := buildTaskContext(contextPassport(), entries[:2])
	if len(messages) != len(without)+3 {
		t.Fatalf("%d messages with the queue step, %d before it; want the call, its result and the finish message added", len(messages), len(without))
	}
	for index := range without {
		if messages[index].Role != without[index].Role || messages[index].Content != without[index].Content {
			t.Fatalf("message %d changed by the queue step", index)
		}
	}
}

func TestOtherStepsGetNoFinishMessage(t *testing.T) {
	for name, entries := range map[string][]ContextEntry{
		"nothing stored yet":             nil,
		"after a read":                   {callEntryFor(2, "read_invoice"), resultEntryFor(2)},
		"after a report was created":     {callEntryFor(4, "create_report"), resultEntryFor(4)},
		"queue_report call, no result":   {callEntryFor(7, "queue_report")},
		"queue_report denied, corrected": {callEntryFor(5, "queue_report"), {StepNumber: 5, Kind: entryCorrection, Content: json.RawMessage(`{"reason_code":"report_export_restricted"}`)}},
		"queued, then a correction": {callEntryFor(7, "queue_report"), resultEntryFor(7),
			{StepNumber: 8, Kind: entryCorrection, Content: json.RawMessage(`{"reason_code":"tool_not_registered"}`)}},
		"queued, then another call": {callEntryFor(7, "queue_report"), resultEntryFor(7), callEntryFor(8, "read_invoice")},
	} {
		messages := buildTaskContext(contextPassport(), entries)
		for _, message := range messages {
			if message.Content == reportQueuedMessage {
				t.Errorf("%s: the finish message was added", name)
			}
		}
	}
}

func TestTheFinishMessageIsTheSameForTheSameStoredSteps(t *testing.T) {
	entries := []ContextEntry{callEntryFor(7, "queue_report"), resultEntryFor(7)}
	first, _ := json.Marshal(buildTaskContext(contextPassport(), entries))
	second, _ := json.Marshal(buildTaskContext(contextPassport(), entries))
	if string(first) != string(second) {
		t.Fatal("the same stored steps built different requests")
	}
}
