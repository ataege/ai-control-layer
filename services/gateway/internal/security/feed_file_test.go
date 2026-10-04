package security

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// committedFeedDigest pins config/attack-signatures.json the way the catalog does; editing the
// file without updating the pin (and the imported row) fails this test.
const committedFeedDigest = "ff6ff4fef7e7091a50c1e416fab5a7b1aa825b55d783ada38de43b42a399d98c"

// committedFeedRevision is the revision label inside config/attack-signatures.json and the one
// config/policy.yaml binds to.
const committedFeedRevision = "feed_v2"

func loadCommittedFeed(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../../../../config", "attack-signatures.json"))
	if err != nil {
		t.Fatalf("read feed: %v", err)
	}
	return raw
}

func TestCommittedFeedParsesWithItsPin(t *testing.T) {
	feed, err := ParseFeed(loadCommittedFeed(t), committedFeedDigest)
	if err != nil {
		t.Fatalf("ParseFeed: %v", err)
	}
	want := []string{
		"prompt_ignore_previous_v1", "prompt_ignore_all_previous_v1", "prompt_ignore_the_previous_v1", "prompt_ignore_prior_v1",
		"prompt_ignore_all_prior_v1", "prompt_disregard_previous_v1", "prompt_disregard_all_previous_v1", "prompt_disregard_prior_v1",
		"code_exec_python_import_v1", "unsafe_deserialization_pickle_v1", "model_repo_trust_remote_code_v1",
	}
	if feed.Issuer != "task-passport-security" || feed.Revision != committedFeedRevision || !slices.Equal(feed.RuleIDs(), want) {
		t.Fatalf("feed = %+v", feed)
	}
	committedPolicy := strings.Replace(samplePolicyContent, `"revision":"feed_v1"`, `"revision":"`+committedFeedRevision+`"`, 1)
	if _, err := SettingsFromCatalog(1, []byte(committedPolicy), loadCommittedFeed(t), committedFeedDigest); err != nil {
		t.Fatalf("sample policy with the committed feed: %v", err)
	}
}

// A copy whose bytes differ from the pinned digest is rejected (the digest stands in for the
// "broken signature" of the roadmap: the feed carries no signature).
func TestCommittedFeedAlteredCopyIsRejected(t *testing.T) {
	altered := append(loadCommittedFeed(t), ' ')
	if _, err := ParseFeed(altered, committedFeedDigest); !errors.Is(err, ErrFeedDigest) {
		t.Fatalf("altered copy: err = %v", err)
	}
}

// On the shared fixtures exactly the labelled cases hit: the two corpus cases holding the report's
// sample phrase and the positive case of each data-only rule. The hostile notes hit nothing.
func TestCommittedFeedOnFixtures(t *testing.T) {
	feed, err := ParseFeed(loadCommittedFeed(t), committedFeedDigest)
	if err != nil {
		t.Fatal(err)
	}
	settings := signatureSettings(1, feed)
	want := map[string]string{
		"indirect_ignore_previous_note_v1": "prompt_ignore_previous_v1",
		"direct_ignore_previous_export_v1": "prompt_ignore_previous_v1",
	}
	got := map[string]string{}
	for _, testCase := range loadCorpus(t) {
		if testCase.SignatureRule != "" {
			want[testCase.ID] = testCase.SignatureRule
		}
		record, err := MatchSignatures(testCase.Text, testCase.Boundary, FieldToolResultText, settings)
		if err != nil {
			t.Fatal(err)
		}
		if record.Outcome == OutcomeBlock {
			if record.FeedDigest != committedFeedDigest {
				t.Fatalf("%s: feed digest %q", testCase.ID, record.FeedDigest)
			}
			got[testCase.ID] = record.MatchedRuleID
		}
	}
	if len(want) != 5 || !maps.Equal(got, want) {
		t.Fatalf("hits = %v, want %v", got, want)
	}
	for _, text := range loadHostileNotes(t) {
		if record, _ := MatchSignatures(text, BoundaryToolResult, FieldInternalNote, settings); record.Outcome != OutcomePass {
			t.Fatalf("hostile note matched %s", record.MatchedRuleID)
		}
	}
}
