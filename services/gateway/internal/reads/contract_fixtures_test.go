package reads

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/provenance"
)

// readContractFixtures are the Go-owned read contracts in packages/contracts (lane w2; NestJS
// consumes them) and the Go type each must decode into exactly. internal/contracts' fixture test
// records them as covered here.
var readContractFixtures = map[string]func() any{
	"run-usage.ledger.json":                          func() any { return new(RunUsage) },
	"run-usage.no-ledger.json":                       func() any { return new(RunUsage) },
	"run-events-page.export-denied.json":             func() any { return new(RunEventPage) },
	"assessment-record.semantic-judge.json":          func() any { return new(AssessmentRecord) },
	"assessment-record.semantic-not-applicable.json": func() any { return new(AssessmentRecord) },
	"assessment-page.two-records.json":               func() any { return new(AssessmentPage) },
	"assessment-page.empty.json":                     func() any { return new(AssessmentPage) },
	"security-event-page.judge.json":                 func() any { return new(SecurityEventPage) },
	"security-summary.judge-split.json":              func() any { return new(SecuritySummary) },
	"catalog-status.active.json":                     func() any { return new(CatalogStatus) },
	"catalog-status.rejected-request.json":           func() any { return new(CatalogStatus) },
	"report-view.vendor.json":                        func() any { return new(provenance.ReportView) },
	"report-view.internal-withheld.json":             func() any { return new(provenance.ReportView) },
}

// TestReadContractFixturesMatchTheGoTypes keeps the Go types and the shared contracts from
// drifting: each fixture decodes strictly (no unknown or missing-type field) and re-encodes to the
// same document, and every stored-record check the reads apply accepts it.
func TestReadContractFixturesMatchTheGoTypes(t *testing.T) {
	for fixtureFile, newTarget := range readContractFixtures {
		t.Run(fixtureFile, func(t *testing.T) {
			fixtureBytes, err := os.ReadFile(filepath.Join(fixturesDirectory, fixtureFile))
			if err != nil {
				t.Fatal(err)
			}
			target := newTarget()
			if err := contracts.DecodeStrict(fixtureBytes, target); err != nil {
				t.Fatalf("fixture does not fit the Go type: %v", err)
			}
			reencoded, err := json.Marshal(target)
			if err != nil {
				t.Fatal(err)
			}
			var fixtureValue, reencodedValue any
			if json.Unmarshal(fixtureBytes, &fixtureValue) != nil || json.Unmarshal(reencoded, &reencodedValue) != nil {
				t.Fatal("not JSON")
			}
			if !reflect.DeepEqual(fixtureValue, reencodedValue) {
				t.Fatalf("round trip changed the document\nfixture:    %s\nre-encoded: %s", fixtureBytes, reencoded)
			}
			switch value := target.(type) {
			case *AssessmentRecord:
				if !validAssessment(*value) {
					t.Fatal("the reads would refuse this assessment")
				}
			case *AssessmentPage:
				for _, record := range value.Records {
					if !validAssessment(record) {
						t.Fatalf("the reads would refuse assessment %s", record.AssessmentID)
					}
				}
				if _, ok := parseWindowCursor(value.NextCursor); !ok {
					t.Fatal("the cursor would be refused")
				}
			case *SecurityEventPage:
				if _, ok := parseWindowCursor(value.NextCursor); !ok {
					t.Fatal("the cursor would be refused")
				}
			}
		})
	}
}
