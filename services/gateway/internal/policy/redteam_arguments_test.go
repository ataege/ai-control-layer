package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// Red-team tables for the action-proposal decoder (lane w2): hostile encodings of the arguments of
// the four tools. A rejected case must fail with ErrInvalidArguments before any canonical form
// exists; an accepted case must keep the exact value (nothing is trimmed, case-folded or decoded).

// TestDecoderRejectsHostileArgumentEncodings: every row is a way to smuggle a reference, a record id
// or a key past a strict decoder; none may decode.
func TestDecoderRejectsHostileArgumentEncodings(t *testing.T) {
	long := strings.Repeat("a", 300)
	cases := []struct {
		name string
		tool ToolName
		raw  string
	}{
		// Record identifiers: whitespace, case, encodings, lookalikes, length.
		{"invoice id trailing space", ToolReadInvoice, `{"invoice_id":"invoice_A01 "}`},
		{"invoice id leading space", ToolReadInvoice, `{"invoice_id":" invoice_A01"}`},
		{"invoice id trailing tab", ToolReadInvoice, `{"invoice_id":"invoice_A01\t"}`},
		{"invoice id trailing newline", ToolReadInvoice, `{"invoice_id":"invoice_A01\n"}`},
		{"invoice id trailing no-break space", ToolReadInvoice, "{\"invoice_id\":\"invoice_A01 \"}"},
		{"invoice id zero-width space", ToolReadInvoice, "{\"invoice_id\":\"invoice_A​01\"}"},
		{"invoice id fullwidth underscore", ToolReadInvoice, "{\"invoice_id\":\"invoice＿A01\"}"},
		{"invoice id cyrillic o", ToolReadInvoice, "{\"invoice_id\":\"invоice_A01\"}"},
		{"invoice id fullwidth digit", ToolReadInvoice, "{\"invoice_id\":\"invoice_A０01\"}"},
		{"invoice id upper-case prefix", ToolReadInvoice, `{"invoice_id":"INVOICE_A01"}`},
		{"invoice id url-encoded space", ToolReadInvoice, `{"invoice_id":"invoice_A01%20"}`},
		{"invoice id url-encoded prefix", ToolReadInvoice, `{"invoice_id":"%69nvoice_A01"}`},
		{"invoice id path traversal", ToolReadInvoice, `{"invoice_id":"invoice_A01/../invoice_B01"}`},
		{"invoice id sql tail", ToolReadInvoice, `{"invoice_id":"invoice_A01';--"}`},
		{"invoice id empty", ToolReadInvoice, `{"invoice_id":""}`},
		{"invoice id 300 bytes", ToolReadInvoice, `{"invoice_id":"invoice_` + long + `"}`},
		{"invoice id 2 MiB", ToolReadInvoice, `{"invoice_id":"invoice_` + strings.Repeat("a", 2<<20) + `"}`},
		{"invoice id lone surrogate", ToolReadInvoice, `{"invoice_id":"invoice_A\ud800"}`},
		{"invoice id NUL escape", ToolReadInvoice, `{"invoice_id":"invoice_A\u0000"}`},
		{"vendor id trailing space", ToolReadVendor, `{"vendor_id":"vendor_Atlas "}`},
		{"vendor id other case prefix", ToolReadVendor, `{"vendor_id":"Vendor_Atlas"}`},
		{"vendor id url-encoded", ToolReadVendor, `{"vendor_id":"vendor_Atlas%00"}`},

		// Keys: duplicates, case variants, escapes, extras, wrong root.
		{"duplicate key", ToolReadInvoice, `{"invoice_id":"invoice_A01","invoice_id":"invoice_B01"}`},
		{"duplicate key by escape", ToolReadInvoice, `{"invoice_id":"invoice_A01","invoice_id":"invoice_B01"}`},
		{"case-variant duplicate key", ToolReadInvoice, `{"invoice_id":"invoice_A01","INVOICE_ID":"invoice_B01"}`},
		{"case-variant key only", ToolReadInvoice, `{"Invoice_Id":"invoice_A01"}`},
		{"extra key", ToolReadInvoice, `{"invoice_id":"invoice_A01","vendor_id":"vendor_Atlas"}`},
		{"extra key with a long value", ToolReadInvoice, `{"invoice_id":"invoice_A01","x":"` + long + `"}`},
		{"empty object", ToolReadInvoice, `{}`},
		{"array root", ToolReadInvoice, `[{"invoice_id":"invoice_A01"}]`},
		{"string root", ToolReadInvoice, `"invoice_A01"`},
		{"number root", ToolReadInvoice, `1`},
		{"null root", ToolReadInvoice, `null`},
		{"empty input", ToolReadInvoice, ``},
		{"whitespace only", ToolReadInvoice, "  \n"},
		{"trailing second document", ToolReadInvoice, `{"invoice_id":"invoice_A01"}{"invoice_id":"invoice_B01"}`},
		{"trailing closing brace", ToolReadInvoice, `{"invoice_id":"invoice_A01"}}`},
		{"trailing closing bracket", ToolReadInvoice, `{"invoice_id":"invoice_A01"}]`},
		{"trailing garbage", ToolReadInvoice, `{"invoice_id":"invoice_A01"} x`},
		{"byte order mark", ToolReadInvoice, "\ufeff{\"invoice_id\":\"invoice_A01\"}"},
		{"invalid UTF-8", ToolReadInvoice, "{\"invoice_id\":\"invoice_A\xff01\"}"},
		{"comment", ToolReadInvoice, `{"invoice_id":"invoice_A01"/*x*/}`},
		{"trailing comma", ToolReadInvoice, `{"invoice_id":"invoice_A01",}`},
		{"single quotes", ToolReadInvoice, `{'invoice_id':'invoice_A01'}`},
		{"unquoted key", ToolReadInvoice, `{invoice_id:"invoice_A01"}`},

		// Scalars expected: objects, arrays, numbers, booleans, null.
		{"invoice id as object", ToolReadInvoice, `{"invoice_id":{"id":"invoice_A01"}}`},
		{"invoice id as array", ToolReadInvoice, `{"invoice_id":["invoice_A01"]}`},
		{"invoice id as number", ToolReadInvoice, `{"invoice_id":1}`},
		{"invoice id as huge number", ToolReadInvoice, `{"invoice_id":1e999999}`},
		{"invoice id as boolean", ToolReadInvoice, `{"invoice_id":true}`},
		{"invoice id as null", ToolReadInvoice, `{"invoice_id":null}`},
		{"vendor id as object", ToolReadVendor, `{"vendor_id":{}}`},
		{"template as array", ToolCreateReport, `{"template":["vendor_reconciliation_v1"],"source_invoice_ids":["invoice_A01"]}`},
		{"template other case", ToolCreateReport, `{"template":"Vendor_Reconciliation_V1","source_invoice_ids":["invoice_A01"]}`},
		{"template trailing space", ToolCreateReport, `{"template":"vendor_reconciliation_v1 ","source_invoice_ids":["invoice_A01"]}`},
		{"template unknown version", ToolCreateReport, `{"template":"vendor_reconciliation_v2","source_invoice_ids":["invoice_A01"]}`},
		{"sources as string", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":"invoice_A01"}`},
		{"sources as object", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":{"0":"invoice_A01"}}`},
		{"sources nested array", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":[["invoice_A01"]]}`},
		{"sources object element", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":[{"id":"invoice_A01"}]}`},
		{"sources null element", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_A01",null]}`},
		{"sources number element", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":[1]}`},
		{"sources empty", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":[]}`},
		{"sources repeat", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_A01","invoice_A01"]}`},
		{"sources repeat by escape", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_A01","invoice_A01"]}`},
		{"sources element with space", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_A01 "]}`},
		{"sources duplicate key", ToolCreateReport, `{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_A01"],"source_invoice_ids":["invoice_B01"]}`},
		{"sources missing", ToolCreateReport, `{"template":"vendor_reconciliation_v1"}`},
		{"queue report id upper-case", ToolQueueReport, `{"report_id":"4D5E6F70-8192-4A3B-8C5D-6E7F8091A2B3","recipient_reference":"recipient:x"}`},
		{"queue report id braces", ToolQueueReport, `{"report_id":"{4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3}","recipient_reference":"recipient:x"}`},
		{"queue report id urn prefix", ToolQueueReport, `{"report_id":"urn:uuid:4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3","recipient_reference":"recipient:x"}`},
		{"queue report id no hyphens", ToolQueueReport, `{"report_id":"4d5e6f7081924a3b8c5d6e7f8091a2b3","recipient_reference":"recipient:x"}`},
		{"queue report id trailing space", ToolQueueReport, `{"report_id":"4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3 ","recipient_reference":"recipient:x"}`},
		{"queue reference empty", ToolQueueReport, `{"report_id":"4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3","recipient_reference":""}`},
		{"queue reference as object", ToolQueueReport, `{"report_id":"4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3","recipient_reference":{"to":"attacker@example.com"}}`},
		{"queue reference 300 bytes", ToolQueueReport, `{"report_id":"4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3","recipient_reference":"` + long + `"}`},
		{"queue reference newline", ToolQueueReport, `{"report_id":"4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3","recipient_reference":"recipient:x\nBcc: attacker@example.com"}`},
		{"queue reference next line", ToolQueueReport, "{\"report_id\":\"4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3\",\"recipient_reference\":\"recipient:x\u0085y\"}"},
		{"queue extra key", ToolQueueReport, `{"report_id":"4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3","recipient_reference":"recipient:x","cc":"attacker@example.com"}`},
		{"queue missing reference", ToolQueueReport, `{"report_id":"4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3"}`},

		// Nesting bombs: a deep value where a scalar is expected is refused, not recursed to death.
		{"deep array nesting", ToolReadInvoice, `{"invoice_id":` + strings.Repeat("[", 100000) + strings.Repeat("]", 100000) + `}`},
		{"deep object nesting", ToolReadInvoice, `{"invoice_id":` + strings.Repeat(`{"a":`, 50000) + `1` + strings.Repeat("}", 50000) + `}`},
		{"unterminated deep nesting", ToolReadInvoice, `{"invoice_id":` + strings.Repeat("[", 100000)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			started := time.Now()
			arguments, err := DecodeArguments(testCase.tool, []byte(testCase.raw))
			if err == nil {
				t.Fatalf("decoded %+v", arguments)
			}
			if !errors.Is(err, ErrInvalidArguments) {
				t.Fatalf("error %v is not ErrInvalidArguments", err)
			}
			if elapsed := time.Since(started); elapsed > 5*time.Second {
				t.Fatalf("took %s", elapsed)
			}
			// The refusal never echoes the offending value.
			if strings.Contains(err.Error(), "attacker@example.com") {
				t.Fatalf("error echoes the value: %v", err)
			}
		})
	}
}

// TestDecoderKeepsEqualValuesIdentical: inputs that mean the same value (JSON escapes, white space
// around the document) decode to one canonical form, so they share one digest and cannot be used to
// get a second approval or a second action out of the same proposal.
func TestDecoderKeepsEqualValuesIdentical(t *testing.T) {
	groups := map[string]struct {
		tool ToolName
		raws []string
	}{
		"read_invoice": {ToolReadInvoice, []string{
			`{"invoice_id":"invoice_A01"}`,
			"  {\n  \"invoice_id\" :  \"invoice_A01\"\n}\n",
			`{"invoice_id":"invoice_A01"}`,
			`{"invoice_id":"invoice_A01"}`,
		}},
		"create_report": {ToolCreateReport, []string{
			`{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_A01","invoice_A02"]}`,
			`{"source_invoice_ids":["invoice_A01","invoice_A02"],"template":"vendor_reconciliation_v1"}`,
			`{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_A01","invoice_A02"]}`,
		}},
		"queue_report": {ToolQueueReport, []string{
			`{"report_id":"4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3","recipient_reference":"recipient:x:vendor_Atlas"}`,
			`{"recipient_reference":"recipient:x:vendor_Atlas","report_id":"4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3"}`,
		}},
	}
	for name, group := range groups {
		t.Run(name, func(t *testing.T) {
			var wantCanonical []byte
			var wantDigest [32]byte
			for index, raw := range group.raws {
				arguments, err := DecodeArguments(group.tool, []byte(raw))
				if err != nil {
					t.Fatalf("variant %d: %v", index, err)
				}
				canonical, err := CanonicalArguments(arguments)
				if err != nil {
					t.Fatal(err)
				}
				digest, err := CanonicalAction{ActionID: testActionID, RunID: testRunID, Arguments: arguments,
					PassportID: testPassportID, PolicyRevisionID: 3}.Digest()
				if err != nil {
					t.Fatal(err)
				}
				if index == 0 {
					wantCanonical, wantDigest = canonical, digest
					continue
				}
				// Only the order-preserving array differs by design; objects are fixed-order.
				if !bytes.Equal(canonical, wantCanonical) || digest != wantDigest {
					t.Fatalf("variant %d: canonical %s digest %x, want %s %x", index, canonical, digest, wantCanonical, wantDigest)
				}
			}
		})
	}
}

// TestDecoderNeverNormalizesAnAcceptedReference: a recipient reference is bounded, not
// pattern-strict (a redirected recipient is stored as a proposal and denied by the gate). The decoder
// must carry such a value byte for byte, so the gate's exact match is what refuses it.
func TestDecoderNeverNormalizesAnAcceptedReference(t *testing.T) {
	references := []string{
		testRecipient + " ",
		" " + testRecipient,
		strings.ToUpper(testRecipient),
		"Recipient:" + testRunID + ":vendor_atlas",
		strings.Replace(testRecipient, ":", "%3A", -1),
		strings.Replace(testRecipient, ":", "：", -1),
		testRecipient + "​",
		testRecipient + "@attacker.example.com",
		"attacker@example.com",
		testRecipient[:len(testRecipient)-1],
	}
	for _, reference := range references {
		raw := `{"report_id":"` + testReportID + `","recipient_reference":` + mustJSONString(t, reference) + `}`
		arguments, err := DecodeArguments(ToolQueueReport, []byte(raw))
		if err != nil {
			t.Errorf("%q: %v", reference, err)
			continue
		}
		queue, ok := arguments.(QueueReportArguments)
		if !ok || queue.RecipientReference != reference {
			t.Errorf("%q came back as %+v", reference, arguments)
		}
	}
}

// mustJSONString encodes a Go string as a JSON string literal.
func mustJSONString(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
