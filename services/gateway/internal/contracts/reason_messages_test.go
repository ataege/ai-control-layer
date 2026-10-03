package contracts

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// GO-58: every X-13 code (the Go list equals the schema enum, TestGoEnumsMatchSchemas) has its own
// safe operator message, and every message can be stored in an event's safeMessage as it is.
func TestEveryReasonCodeHasASafeMessage(t *testing.T) {
	var schema struct {
		Enum []ReasonCode `json:"enum"`
	}
	if err := json.Unmarshal(readFile(t, filepath.Join(schemasDirectory, "reason-code.schema.json")), &schema); err != nil {
		t.Fatal(err)
	}
	seen := map[string]ReasonCode{}
	for _, code := range schema.Enum {
		message := code.SafeMessage()
		if message == genericReasonMessage {
			t.Errorf("%s has no message of its own", code)
		}
		if utf8.RuneCountInString(message) > 512 || strings.ContainsAny(message, "%{}<>") ||
			strings.ContainsFunc(message, unicode.IsControl) {
			t.Errorf("%s: message cannot be stored as a safe message: %q", code, message)
		}
		if other, duplicate := seen[message]; duplicate {
			t.Errorf("%s and %s share a message", code, other)
		}
		seen[message] = code
	}
	for code := range reasonMessages {
		if !slices.Contains(schema.Enum, code) {
			t.Errorf("message for %s, which is not an X-13 code", code)
		}
	}
	if ReasonCode("made_up").SafeMessage() != genericReasonMessage {
		t.Fatal("a value outside the vocabulary got a specific message")
	}
	var fixtureCode ReasonCode
	if err := json.Unmarshal(readFile(t, filepath.Join(fixturesDirectory, "reason-code.report-export-restricted.json")), &fixtureCode); err != nil ||
		fixtureCode.SafeMessage() == genericReasonMessage {
		t.Fatalf("the reason-code fixture %q has no message", fixtureCode)
	}
}
