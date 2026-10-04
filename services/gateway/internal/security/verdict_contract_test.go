package security

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// The shared X-79 verdict contract (packages/contracts, SH-11) must be exactly the output format
// Go sends to the model, and its fixture must pass Go's own parser.
const (
	verdictSchemaFile  = "../../../../packages/contracts/schemas/semantic-verdict.schema.json"
	verdictFixtureFile = "../../../../packages/contracts/fixtures/semantic-verdict.instruction-injection.json"
)

func TestSharedVerdictContractIsTheModelOutputFormat(t *testing.T) {
	schemaBytes, err := os.ReadFile(verdictSchemaFile)
	if err != nil {
		t.Fatalf("read the shared schema: %v", err)
	}
	var shared map[string]any
	if err := json.Unmarshal(schemaBytes, &shared); err != nil {
		t.Fatalf("decode the shared schema: %v", err)
	}
	var format map[string]any
	if err := json.Unmarshal(verdictFormat, &format); err != nil {
		t.Fatalf("decode verdictFormat: %v", err)
	}
	// The shared schema adds only its descriptive keys; every validating keyword is Go's.
	for _, key := range []string{"type", "properties", "required", "additionalProperties"} {
		if !reflect.DeepEqual(shared[key], format[key]) {
			t.Errorf("%s differs:\nshared %v\nGo     %v", key, shared[key], format[key])
		}
	}

	fixture, err := os.ReadFile(verdictFixtureFile)
	if err != nil {
		t.Fatalf("read the fixture: %v", err)
	}
	verdict, err := ParseVerdict(string(fixture))
	if err != nil {
		t.Fatalf("ParseVerdict rejected the shared fixture: %v", err)
	}
	if verdict != (Verdict{RiskCategory: "instruction_injection", Score: 0.92, ReasonCode: "instruction_override"}) {
		t.Errorf("parsed %+v", verdict)
	}
}
