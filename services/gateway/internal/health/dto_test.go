package health

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const fixturesDirectory = "../../../../packages/contracts/fixtures"

// TestDTOsMatchContractFixtures is the Go half of the contract check: every
// shared fixture must decode strictly and re-encode to equivalent JSON.
func TestDTOsMatchContractFixtures(t *testing.T) {
	tests := []struct {
		fixtureFile string
		newTarget   func() any
	}{
		{"liveness.gateway.json", func() any { return new(LivenessResponse) }},
		{"liveness.api.json", func() any { return new(LivenessResponse) }},
		{"readiness.ready.json", func() any { return new(ReadinessResponse) }},
		{"readiness.unavailable.json", func() any { return new(ReadinessResponse) }},
		{"gateway-ping.ok.json", func() any { return new(GatewayPingResponse) }},
		{"error.unauthorized.json", func() any { return new(ErrorResponse) }},
	}
	for _, testCase := range tests {
		t.Run(testCase.fixtureFile, func(t *testing.T) {
			fixtureBytes, err := os.ReadFile(filepath.Join(fixturesDirectory, testCase.fixtureFile))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			decoded := testCase.newTarget()
			strictDecoder := json.NewDecoder(bytes.NewReader(fixtureBytes))
			strictDecoder.DisallowUnknownFields()
			if err := strictDecoder.Decode(decoded); err != nil {
				t.Fatalf("fixture does not fit the Go DTO: %v", err)
			}

			reencodedBytes, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("re-encode: %v", err)
			}
			var fixtureValue, reencodedValue any
			if err := json.Unmarshal(fixtureBytes, &fixtureValue); err != nil {
				t.Fatalf("fixture is not valid JSON: %v", err)
			}
			if err := json.Unmarshal(reencodedBytes, &reencodedValue); err != nil {
				t.Fatalf("re-encoded DTO is not valid JSON: %v", err)
			}
			// Catches fields the DTO drops, renames or adds.
			if !reflect.DeepEqual(fixtureValue, reencodedValue) {
				t.Errorf("round trip changed the document\nfixture:    %s\nre-encoded: %s", fixtureBytes, reencodedBytes)
			}
		})
	}
}

// TestEmittedValuesMatchContractFixtures ties the constants the handlers emit
// to the fixture values, which the round trip above cannot see.
func TestEmittedValuesMatchContractFixtures(t *testing.T) {
	tests := []struct {
		fixtureFile string
		expected    ReadinessResponse
	}{
		{"liveness.gateway.json", ReadinessResponse{Status: StatusOK, Service: ServiceName}},
		{"gateway-ping.ok.json", ReadinessResponse{Status: StatusOK, Service: ServiceName}},
		{"readiness.ready.json", ReadinessResponse{
			Status: StatusOK, Service: ServiceName,
			Checks: ReadinessChecks{Database: DependencyCheck{Status: CheckUp}},
		}},
		{"readiness.unavailable.json", ReadinessResponse{
			Status: StatusUnavailable, Service: ServiceName,
			Checks: ReadinessChecks{Database: DependencyCheck{Status: CheckDown}},
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.fixtureFile, func(t *testing.T) {
			fixtureBytes, err := os.ReadFile(filepath.Join(fixturesDirectory, testCase.fixtureFile))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			// Liveness and ping fixtures simply have no checks.
			var decoded ReadinessResponse
			if err := json.Unmarshal(fixtureBytes, &decoded); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			// The message is free text, not part of the value contract.
			decoded.Checks.Database.Message = ""
			if decoded != testCase.expected {
				t.Errorf("fixture values = %+v, handlers emit %+v", decoded, testCase.expected)
			}
		})
	}
}
