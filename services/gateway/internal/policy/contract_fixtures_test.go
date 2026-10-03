package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"starter/services/gateway/internal/contracts"
)

// reviewContractFixtures are the Go-owned review and approval contracts in packages/contracts (lane
// w3; NestJS consumes them), each with the Go type its route encodes. internal/contracts' fixture
// test records them as covered here.
var reviewContractFixtures = map[string]func() any{
	"review-view.queue-report.json":  func() any { return new(ReviewPayload) },
	"approval-response.approve.json": func() any { return new(approvalResponse) },
	"approval-response.reject.json":  func() any { return new(approvalResponse) },
}

// TestReviewContractFixturesMatchTheGoTypes keeps the Go types and the shared contracts from
// drifting: each fixture decodes strictly into the type ReviewHandler or ApprovalHandler encodes,
// re-encodes to the same document, and a review payload still yields its canonical digest.
func TestReviewContractFixturesMatchTheGoTypes(t *testing.T) {
	for fixtureFile, newTarget := range reviewContractFixtures {
		t.Run(fixtureFile, func(t *testing.T) {
			fixtureBytes, err := os.ReadFile(filepath.Join("../../../../packages/contracts/fixtures", fixtureFile))
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
			case *ReviewPayload:
				if value.Recipient == nil || value.Report == nil || value.Tool != ToolQueueReport {
					t.Fatalf("a queue_report review needs its recipient and report: %+v", value)
				}
				if _, _, err := value.Digest(); err != nil {
					t.Fatalf("the review payload has no canonical digest: %v", err)
				}
			case *approvalResponse:
				if value.Decision != ApprovalApprove && value.Decision != ApprovalReject {
					t.Fatalf("decision %q", value.Decision)
				}
			}
		})
	}
}
