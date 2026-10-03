package policy

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestEquivalentArgumentsGiveIdenticalCanonicalBytes(t *testing.T) {
	cases := []struct {
		name        string
		tool        ToolName
		equivalents []string
		want        string
	}{
		{
			name:        "read_invoice: whitespace and escapes",
			tool:        ToolReadInvoice,
			equivalents: []string{`{"invoice_id":"invoice_A01"}`, " {\n  \"invoice_id\" : \"invoice_A01\"\n} ", `{"invoice_id":"invoice_A01"}`},
			want:        `{"invoice_id":"invoice_A01"}`,
		},
		{
			name:        "read_vendor",
			tool:        ToolReadVendor,
			equivalents: []string{`{"vendor_id":"vendor_atlas"}`, `{ "vendor_id":"vendor_atlas" }`},
			want:        `{"vendor_id":"vendor_atlas"}`,
		},
		{
			name: "create_report: field order",
			tool: ToolCreateReport,
			equivalents: []string{
				`{"template":"internal_investigation_v1","invoice_ids":["invoice_A01","invoice_A02"]}`,
				`{"invoice_ids":["invoice_A01","invoice_A02"],"template":"internal_investigation_v1"}`,
			},
			want: `{"template":"internal_investigation_v1","invoice_ids":["invoice_A01","invoice_A02"]}`,
		},
		{
			name: "queue_report: field order",
			tool: ToolQueueReport,
			equivalents: []string{
				`{"report_id":"6f1c2a3b-0000-4000-8000-000000000001","recipient_reference":"atlas_reporting"}`,
				`{"recipient_reference":"atlas_reporting","report_id":"6f1c2a3b-0000-4000-8000-000000000001"}`,
			},
			want: `{"report_id":"6f1c2a3b-0000-4000-8000-000000000001","recipient_reference":"atlas_reporting"}`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, rawArguments := range testCase.equivalents {
				arguments, err := DecodeArguments(testCase.tool, []byte(rawArguments))
				if err != nil {
					t.Fatalf("decode %s: %v", rawArguments, err)
				}
				canonicalBytes, err := CanonicalArguments(arguments)
				if err != nil {
					t.Fatalf("canonicalize: %v", err)
				}
				if string(canonicalBytes) != testCase.want {
					t.Fatalf("canonical bytes = %s, want %s", canonicalBytes, testCase.want)
				}
			}
		})
	}
}

func TestListOrderIsPreserved(t *testing.T) {
	first, _ := DecodeArguments(ToolCreateReport, []byte(`{"template":"vendor_reconciliation_v1","invoice_ids":["invoice_A01","invoice_A02"]}`))
	second, _ := DecodeArguments(ToolCreateReport, []byte(`{"template":"vendor_reconciliation_v1","invoice_ids":["invoice_A02","invoice_A01"]}`))
	firstBytes, _ := CanonicalArguments(first)
	secondBytes, _ := CanonicalArguments(second)
	if bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("reordered invoice_ids gave the same canonical bytes")
	}
}

func TestAmbiguousOrUnsupportedArgumentsAreRejected(t *testing.T) {
	cases := []struct {
		name         string
		tool         ToolName
		rawArguments string
	}{
		{"unknown field", ToolReadInvoice, `{"invoice_id":"invoice_A01","note":"x"}`},
		{"duplicate field", ToolReadInvoice, `{"invoice_id":"invoice_A01","invoice_id":"invoice_B01"}`},
		{"case variant of a field", ToolReadInvoice, `{"INVOICE_ID":"invoice_A01"}`},
		{"field plus case variant", ToolReadInvoice, `{"invoice_id":"invoice_A01","Invoice_Id":"invoice_B01"}`},
		{"missing field", ToolCreateReport, `{"template":"internal_investigation_v1"}`},
		{"null field", ToolReadVendor, `{"vendor_id":null}`},
		{"null list item", ToolCreateReport, `{"template":"internal_investigation_v1","invoice_ids":[null]}`},
		{"number instead of string", ToolReadInvoice, `{"invoice_id":5}`},
		{"float in a list", ToolCreateReport, `{"template":"internal_investigation_v1","invoice_ids":[1.5]}`},
		{"object instead of string", ToolReadVendor, `{"vendor_id":{"id":"vendor_atlas"}}`},
		{"empty string", ToolReadInvoice, `{"invoice_id":""}`},
		{"control character", ToolReadInvoice, `{"invoice_id":"invoice\u0000A01"}`},
		{"identifier too long", ToolReadInvoice, `{"invoice_id":"` + string(bytes.Repeat([]byte("a"), maximumIdentifierBytes+1)) + `"}`},
		{"invalid UTF-8", ToolReadInvoice, "{\"invoice_id\":\"invoice\xff\"}"},
		{"trailing data", ToolReadInvoice, `{"invoice_id":"invoice_A01"} {}`},
		{"top-level array", ToolReadInvoice, `[{"invoice_id":"invoice_A01"}]`},
		{"malformed JSON", ToolReadInvoice, `{"invoice_id":`},
		{"unregistered template", ToolCreateReport, `{"template":"public_summary_v1","invoice_ids":["invoice_A01"]}`},
		{"empty invoice list", ToolCreateReport, `{"template":"internal_investigation_v1","invoice_ids":[]}`},
		{"repeated invoice", ToolCreateReport, `{"template":"internal_investigation_v1","invoice_ids":["invoice_A01","invoice_A01"]}`},
		{"report_id not a UUID", ToolQueueReport, `{"report_id":"report-1","recipient_reference":"atlas_reporting"}`},
		{"uppercase UUID", ToolQueueReport, `{"report_id":"6F1C2A3B-0000-4000-8000-000000000001","recipient_reference":"atlas_reporting"}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := DecodeArguments(testCase.tool, []byte(testCase.rawArguments))
			if !errors.Is(err, ErrInvalidArguments) {
				t.Fatalf("error = %v, want ErrInvalidArguments", err)
			}
		})
	}
}

func TestUnregisteredToolIsRejected(t *testing.T) {
	_, err := DecodeArguments("send_email", []byte(`{"to":"x"}`))
	if !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("error = %v, want ErrUnknownTool", err)
	}
}

func baseAction(t *testing.T) CanonicalAction {
	t.Helper()
	arguments, err := DecodeArguments(ToolQueueReport, []byte(`{"report_id":"6f1c2a3b-0000-4000-8000-000000000001","recipient_reference":"atlas_reporting"}`))
	if err != nil {
		t.Fatal(err)
	}
	recipient := "reports@atlas.example"
	content := "INV104 appears on invoice_A01 and invoice_A02."
	expiresAt := time.Date(2026, 10, 3, 18, 15, 0, 123456789, time.UTC)
	return CanonicalAction{
		ActionID:          "11111111-1111-4111-8111-111111111111",
		RunID:             "22222222-2222-4222-8222-222222222222",
		Arguments:         arguments,
		PassportID:        "33333333-3333-4333-8333-333333333333",
		PolicyRevisionID:  7,
		Recipient:         &recipient,
		AffectedResources: []ResourceVersion{{Kind: "invoice", ID: "invoice_A01", Version: 1}, {Kind: "invoice", ID: "invoice_A02", Version: 1}},
		OutboundContent:   &content,
		ExpiresAt:         &expiresAt,
	}
}

func TestDigestIsStableAcrossTimeZones(t *testing.T) {
	action := baseAction(t)
	firstDigest, err := action.Digest()
	if err != nil {
		t.Fatal(err)
	}
	sameInstantElsewhere := action.ExpiresAt.In(time.FixedZone("CEST", 2*60*60))
	action.ExpiresAt = &sameInstantElsewhere
	secondDigest, _ := action.Digest()
	if firstDigest != secondDigest {
		t.Fatal("the same expiry instant in another zone changed the digest")
	}
}

func TestChangingAnyMaterialFieldChangesTheDigest(t *testing.T) {
	baseDigest, err := baseAction(t).Digest()
	if err != nil {
		t.Fatal(err)
	}
	otherArguments, _ := DecodeArguments(ToolQueueReport, []byte(`{"report_id":"6f1c2a3b-0000-4000-8000-000000000001","recipient_reference":"other_vendor"}`))
	otherTool, _ := DecodeArguments(ToolReadInvoice, []byte(`{"invoice_id":"invoice_A01"}`))
	changes := map[string]func(*CanonicalAction){
		"action_id":          func(a *CanonicalAction) { a.ActionID = "11111111-1111-4111-8111-111111111112" },
		"run_id":             func(a *CanonicalAction) { a.RunID = "22222222-2222-4222-8222-222222222223" },
		"arguments":          func(a *CanonicalAction) { a.Arguments = otherArguments },
		"tool":               func(a *CanonicalAction) { a.Arguments = otherTool },
		"passport_id":        func(a *CanonicalAction) { a.PassportID = "33333333-3333-4333-8333-333333333334" },
		"policy_revision_id": func(a *CanonicalAction) { a.PolicyRevisionID = 8 },
		"recipient":          func(a *CanonicalAction) { other := "billing@atlas.example"; a.Recipient = &other },
		"recipient absent":   func(a *CanonicalAction) { a.Recipient = nil },
		"resource version":   func(a *CanonicalAction) { a.AffectedResources[1].Version = 2 },
		"resource order": func(a *CanonicalAction) {
			a.AffectedResources[0], a.AffectedResources[1] = a.AffectedResources[1], a.AffectedResources[0]
		},
		"outbound content":  func(a *CanonicalAction) { other := *a.OutboundContent + " "; a.OutboundContent = &other },
		"content empty":     func(a *CanonicalAction) { empty := ""; a.OutboundContent = &empty },
		"content absent":    func(a *CanonicalAction) { a.OutboundContent = nil },
		"expiry nanosecond": func(a *CanonicalAction) { later := a.ExpiresAt.Add(time.Nanosecond); a.ExpiresAt = &later },
		"expiry absent":     func(a *CanonicalAction) { a.ExpiresAt = nil },
	}
	for changeName, applyChange := range changes {
		t.Run(changeName, func(t *testing.T) {
			changedAction := baseAction(t)
			applyChange(&changedAction)
			changedDigest, err := changedAction.Digest()
			if err != nil {
				t.Fatal(err)
			}
			if changedDigest == baseDigest {
				t.Fatalf("changing %s did not change the digest", changeName)
			}
		})
	}
}

func TestInvalidActionFieldsAreRejected(t *testing.T) {
	changes := map[string]func(*CanonicalAction){
		"action_id not a UUID":  func(a *CanonicalAction) { a.ActionID = "action-1" },
		"missing arguments":     func(a *CanonicalAction) { a.Arguments = nil },
		"zero policy revision":  func(a *CanonicalAction) { a.PolicyRevisionID = 0 },
		"empty recipient":       func(a *CanonicalAction) { empty := ""; a.Recipient = &empty },
		"zero resource version": func(a *CanonicalAction) { a.AffectedResources[0].Version = 0 },
		"invalid UTF-8 content": func(a *CanonicalAction) { invalid := "bad\xff"; a.OutboundContent = &invalid },
	}
	for changeName, applyChange := range changes {
		t.Run(changeName, func(t *testing.T) {
			action := baseAction(t)
			applyChange(&action)
			if _, err := action.Digest(); !errors.Is(err, ErrInvalidAction) {
				t.Fatalf("error = %v, want ErrInvalidAction", err)
			}
		})
	}
}

func TestAbsentAndEmptyResourcesShareOneRepresentation(t *testing.T) {
	withNil := baseAction(t)
	withNil.AffectedResources = nil
	withEmpty := baseAction(t)
	withEmpty.AffectedResources = []ResourceVersion{}
	nilBytes, _ := withNil.Encode()
	emptyBytes, _ := withEmpty.Encode()
	if !bytes.Equal(nilBytes, emptyBytes) {
		t.Fatalf("nil and empty resources encode differently: %s vs %s", nilBytes, emptyBytes)
	}
}
