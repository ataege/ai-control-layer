package policy

import (
	"context"
	"testing"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/contracts"
)

// limitingTools is a catalog snapshot that sets only the tool-attempt ceiling.
func limitingTools(revision, toolAttempts int64) fakeSnapshots {
	return fakeSnapshots{snapshot: catalog.Snapshot{RevisionID: revision, Limits: catalog.Limits{
		ToolAttempts: toolAttempts, EnabledTemplates: []contracts.ReportTemplate{contracts.TemplateVendorReconciliation},
	}}}
}

// config/README.md: a lowered budget constrains running tasks, and a raised one never exceeds the
// stored passport (12 tool attempts in the review world).
func TestToolAttemptLimitFollowsTheActiveCatalogButNeverWidensThePassport(t *testing.T) {
	world := openReviewWorld(t)
	for name, testCase := range map[string]struct {
		catalogLimit int64
		want         int
	}{
		"lowered by the catalog":  {catalogLimit: 4, want: 4},
		"raised above passport":   {catalogLimit: 100, want: 12},
		"equal to the passport":   {catalogLimit: 12, want: 12},
		"lowered to a single try": {catalogLimit: 1, want: 1},
	} {
		scope, err := scopeReaderOn(world, limitingTools(2, testCase.catalogLimit)).LoadScope(context.Background(), world.run)
		if err != nil || scope.ToolAttemptLimit != testCase.want {
			t.Errorf("%s: tool attempt limit %d err %v, want %d", name, scope.ToolAttemptLimit, err, testCase.want)
		}
	}
}
