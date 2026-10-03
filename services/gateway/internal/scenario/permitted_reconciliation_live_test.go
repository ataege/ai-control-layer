//go:build model_live

package scenario

import (
	"os"
	"testing"

	"starter/services/gateway/internal/config"
)

// TestLivePermittedReconciliation runs beats 3 and 4 with the live local model (decision 6),
// labelled live. The model chooses its own steps, so the test checks what it did: every returned
// field is allowlisted and in scope, and the internal report it created is Internal only with
// passport sources and the INV104 finding. Needs -tags=model_live, GO_STORY_LIVE=1,
// MODEL_BASE_URL, MODEL_NAME (allowed by the seeded catalog) and the test database.
func TestLivePermittedReconciliation(t *testing.T) {
	if os.Getenv("GO_STORY_LIVE") != "1" {
		t.Skip("live story skipped: set GO_STORY_LIVE=1")
	}
	modelConfig, err := config.LoadModel()
	if err != nil {
		t.Fatalf("model configuration: %v", err)
	}
	world := openStory(t, &modelConfig)
	world.runQueuedJob(t)
	status, reason := world.runStatus(t)
	fieldEvidence := assertReturnedFieldsObeyPolicy(t, world, world.returnedResults(t))
	reportEvidence := assertInternalReportIsPermitted(t, world)
	t.Logf("evidence GO-27 (X-63, live %s): run %s %s; returned fields %v; %s",
		modelConfig.Name, status, reason, fieldEvidence, reportEvidence)
}
