package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"starter/services/gateway/internal/contracts"
)

type bodyTarget struct {
	Decision string `json:"decision"`
}

// TestBodyDecoderRefusesEverythingAfterTheDocument (lane w2): one document, then the end of the
// input. A stray closing brace or bracket after the document used to pass `decoder.More`.
func TestBodyDecoderRefusesEverythingAfterTheDocument(t *testing.T) {
	for name, body := range map[string]string{
		"closing brace":     `{"decision":"approve"}}`,
		"closing bracket":   `{"decision":"approve"}]`,
		"several braces":    `{"decision":"approve"}}}}`,
		"second document":   `{"decision":"approve"}{"decision":"reject"}`,
		"garbage":           `{"decision":"approve"} x`,
		"comma":             `{"decision":"approve"},`,
		"nul":               "{\"decision\":\"approve\"}\x00",
		"second line":       "{\"decision\":\"approve\"}\n{}",
		"byte order mark":   "\ufeff{\"decision\":\"approve\"}",
		"leading garbage":   `x{"decision":"approve"}`,
		"array":             `[{"decision":"approve"}]`,
		"empty":             ``,
		"truncated":         `{"decision":"approve"`,
		"unknown field":     `{"decision":"approve","grant":true}`,
		"nesting bomb":      `{"decision":` + strings.Repeat("[", 100000) + strings.Repeat("]", 100000) + `}`,
		"number for string": `{"decision":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			var target bodyTarget
			err := decodeJSONBody(httptest.NewRecorder(), httptest.NewRequest("POST", "/x", strings.NewReader(body)), 1<<20, &target)
			if err == nil {
				t.Fatalf("accepted %q as %+v", body, target)
			}
		})
	}
	t.Run("document with surrounding white space is accepted", func(t *testing.T) {
		var target bodyTarget
		body := " \n{\"decision\":\"approve\"}\r\n\t "
		if err := decodeJSONBody(httptest.NewRecorder(), httptest.NewRequest("POST", "/x", strings.NewReader(body)), 1<<20, &target); err != nil || target.Decision != "approve" {
			t.Fatalf("%v %+v", err, target)
		}
	})
}

// TestCommandBodiesRefuseDuplicateAndCaseVariantKeys: the four internal commands decode their bodies
// into the contracts' types. A repeated key (last would win), a case variant (matched
// case-insensitively), or either of them nested, is a bad request, and the 400 never echoes it.
func TestCommandBodiesRefuseDuplicateAndCaseVariantKeys(t *testing.T) {
	type target func() any
	approval := func() any { return &contracts.ApprovalDecision{} }
	startRun := func() any { return &contracts.StartRunRequest{} }
	evaluate := func() any { return &contracts.ControlEvaluationRequest{} }
	cancel := func() any { return &struct{}{} }
	cases := []struct {
		name string
		make target
		body string
	}{
		{"approval duplicate", approval, `{"decision":"reject","decision":"approve"}`},
		{"approval duplicate by escape", approval, `{"decision":"reject","decisio\u006e":"approve"}`},
		{"approval upper-case key", approval, `{"DECISION":"approve"}`},
		{"approval capitalized key", approval, `{"Decision":"approve"}`},
		{"approval case-variant pair", approval, `{"decision":"reject","Decision":"approve"}`},
		{"start-run duplicate", startRun, `{"template":"a","template":"b","invoiceIds":["invoice_A01"],"destination":"d"}`},
		{"start-run case variant", startRun, `{"Template":"a","invoiceIds":["invoice_A01"],"destination":"d"}`},
		{"start-run nested duplicate", startRun, `{"template":"a","invoiceIds":["invoice_A01"],"destination":"d","limits":{"modelCalls":1,"modelCalls":999}}`},
		{"start-run nested case variant", startRun, `{"template":"a","invoiceIds":["invoice_A01"],"destination":"d","limits":{"MODELCALLS":999}}`},
		{"evaluate duplicate kind", evaluate, `{"runId":"r","kind":"model_input","kind":"action_proposal","text":"t","tool":null,"arguments":null}`},
		{"evaluate case-variant runId", evaluate, `{"RUNID":"r","kind":"model_input","text":"t","tool":null,"arguments":null}`},
		{"evaluate duplicate inside arguments", evaluate, `{"runId":"r","kind":"action_proposal","text":null,"tool":"read_invoice","arguments":{"invoice_id":"invoice_A01","invoice_id":"invoice_B01"}}`},
		{"evaluate duplicate deep inside arguments", evaluate, `{"runId":"r","kind":"action_proposal","text":null,"tool":"read_invoice","arguments":{"a":{"b":[{"c":1,"c":2}]}}}`},
		{"cancel duplicate", cancel, `{"x":1,"x":2}`},
		{"nesting past the bound", approval, `{"decision":` + strings.Repeat("[", 100) + strings.Repeat("]", 100) + `}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(testCase.body))
			if DecodeJSONBody(recorder, request, 1<<20, testCase.make()) {
				t.Fatal("accepted")
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status %d", recorder.Code)
			}
			for _, echoed := range []string{"approve", "reject", "999", "invoice_B01", "DECISION", "MODELCALLS"} {
				if strings.Contains(recorder.Body.String(), echoed) {
					t.Fatalf("the 400 echoes %q: %s", echoed, recorder.Body.String())
				}
			}
		})
	}
}

// TestCommandBodiesStillAcceptTheirOwnShape: the exact spelling of every field of the four
// commands decodes, so the stricter check refuses nothing a real caller sends.
func TestCommandBodiesStillAcceptTheirOwnShape(t *testing.T) {
	var approval contracts.ApprovalDecision
	var startRun contracts.StartRunRequest
	var evaluation contracts.ControlEvaluationRequest
	for name, check := range map[string]struct {
		target any
		body   string
	}{
		"approval":             {&approval, `{"decision":"approve"}`},
		"start-run":            {&startRun, `{"template":"reconcile_atlas_v1","vendorId":"vendor_Atlas","invoiceIds":["invoice_A01","invoice_A02"],"destination":"registered_vendor","approvalRequirement":"required","limits":{"modelCalls":10,"timeoutSeconds":300}}`},
		"evaluate model input": {&evaluation, `{"runId":"r","kind":"model_input","text":"hello","tool":null,"arguments":null}`},
		"evaluate proposal":    {&evaluation, `{"runId":"r","kind":"action_proposal","text":null,"tool":"read_invoice","arguments":{"invoice_id":"invoice_A01"}}`},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(check.body))
			if !DecodeJSONBody(recorder, request, 1<<20, check.target) {
				t.Fatalf("refused: %s", recorder.Body.String())
			}
		})
	}
}
