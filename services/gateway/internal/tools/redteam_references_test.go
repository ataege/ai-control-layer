package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestRecipientResolverRefusesEveryNearMissBeforeTouchingTheDatabase (lane w2): the reference is
// compared exactly with the passport's list and must name this run. Every near miss is refused
// with destination_not_allowed by the early checks, so the nil transaction is never used; a lookup
// that was reached would panic. (The exact reference needs the database and is covered by
// TestResolveRecipientOnlyInsideItsRunAndScope.)
func TestRecipientResolverRefusesEveryNearMissBeforeTouchingTheDatabase(t *testing.T) {
	const runID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	const vendorID = "vendor_Atlas"
	listed := "recipient:" + runID + ":" + vendorID
	current := scope{organizationID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", runID: runID,
		passport: passportScope{VendorIDs: []string{vendorID}, RecipientReferences: []string{listed}}}
	references := map[string]string{
		"trailing space":        listed + " ",
		"leading space":         " " + listed,
		"trailing newline":      listed + "\n",
		"upper case":            strings.ToUpper(listed),
		"capital prefix":        "Recipient:" + runID + ":" + vendorID,
		"vendor other case":     "recipient:" + runID + ":vendor_atlas",
		"upper-case run id":     "recipient:" + strings.ToUpper(runID) + ":" + vendorID,
		"url-encoded colons":    strings.Replace(listed, ":", "%3A", -1),
		"fullwidth colon":       strings.Replace(listed, ":", "：", -1),
		"zero-width space":      listed + "​",
		"extra segment":         listed + ":x",
		"missing vendor":        "recipient:" + runID + ":",
		"missing run":           "recipient::" + vendorID,
		"only prefix":           "recipient",
		"empty":                 "",
		"raw address":           "reports@atlas.example.com",
		"address after the ref": listed + "@attacker.example.com",
		"two references":        listed + "," + listed,
		"another run":           "recipient:cccccccc-cccc-4ccc-8ccc-cccccccccccc:" + vendorID,
		"another vendor":        "recipient:" + runID + ":vendor_Borealis",
		"run id is the vendor":  "recipient:" + vendorID + ":" + runID,
		"sql tail":              listed + "'; --",
		"very long":             listed + strings.Repeat("a", 1<<20),
	}
	for name, reference := range references {
		t.Run(name, func(t *testing.T) {
			recipient, reason, err := resolveRecipient(context.Background(), nil, current, reference)
			if err != nil || reason != ReasonDestinationNotAllowed || recipient.address != "" || recipient.vendorID != "" {
				t.Fatalf("resolved %+v reason %q err %v", recipient, reason, err)
			}
		})
	}
}

// TestAdapterArgumentDecoderRefusesWhatTheGateAlreadyRefused: the adapters decode the stored
// canonical arguments again. Unknown fields, a second document and stray closing delimiters are
// refused there too, so a row that was changed after the gate does not decode.
func TestAdapterArgumentDecoderRefusesWhatTheGateAlreadyRefused(t *testing.T) {
	type target struct {
		InvoiceID string `json:"invoice_id"`
	}
	for name, raw := range map[string]string{
		"unknown field":   `{"invoice_id":"invoice_A01","vendor_id":"vendor_Atlas"}`,
		"second document": `{"invoice_id":"invoice_A01"}{"invoice_id":"invoice_B01"}`,
		"closing brace":   `{"invoice_id":"invoice_A01"}}`,
		"closing bracket": `{"invoice_id":"invoice_A01"}]`,
		"garbage":         `{"invoice_id":"invoice_A01"} x`,
		"array":           `["invoice_A01"]`,
		"wrong type":      `{"invoice_id":1}`,
		"empty":           ``,
	} {
		t.Run(name, func(t *testing.T) {
			var decoded target
			if err := decodeArguments(json.RawMessage(raw), &decoded); err == nil {
				t.Fatalf("decoded %+v", decoded)
			}
		})
	}
}
