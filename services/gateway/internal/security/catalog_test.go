package security

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// samplePolicyContent is config/policy.yaml as the catalog stores it (JSON content).
const samplePolicyContent = `{"schema_version":1,"allowed_models":["qwen3.5:4b"],"budgets":{"calls_total":24,"tokens_total":20000},
"controls":{
  "secret_pattern":{"enabled":true,"mode":"redact","boundaries":["model_input","tool_result"]},
  "semantic_injection":{"enabled":true,"mode":"block","threshold":0.75,"boundaries":["model_input","tool_result","action_proposal"]},
  "signature_match":{"enabled":true,"boundaries":["model_input","tool_result","action_proposal"]}},
"signatures":{"path":"attack-signatures.json","revision":"feed_v1","disabled_rules":[]},
"reports":{"enabled_templates":["internal_investigation_v1","vendor_reconciliation_v1"]}}`

func TestSettingsFromCatalogReadsSamplePolicy(t *testing.T) {
	feed := feedJSON("feed_v1", sampleRule)
	settings, err := SettingsFromCatalog(3, []byte(samplePolicyContent), []byte(feed), digestOf(feed))
	if err != nil {
		t.Fatal(err)
	}
	all := []Boundary{BoundaryModelInput, BoundaryToolResult, BoundaryActionProposal}
	if settings.EvaluatedCatalogRevisionID != 3 || !settings.SecretPattern.Enabled || settings.SecretPattern.Mode != ModeRedact ||
		!slices.Equal(settings.SecretPattern.Boundaries, []Boundary{BoundaryModelInput, BoundaryToolResult}) ||
		settings.SemanticInjection.Mode != ModeBlock || settings.SemanticInjection.Threshold != 0.75 ||
		!slices.Equal(settings.SemanticInjection.Boundaries, all) || !slices.Equal(settings.SignatureMatch.Boundaries, all) ||
		settings.Feed == nil || settings.Feed.Revision != "feed_v1" || len(settings.DisabledRules) != 0 {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestSettingsFromCatalogDisabledSignaturesNeedNoFeed(t *testing.T) {
	content := strings.Replace(samplePolicyContent, `"signature_match":{"enabled":true`, `"signature_match":{"enabled":false`, 1)
	settings, err := SettingsFromCatalog(3, []byte(content), nil, "")
	if err != nil || settings.Feed != nil || settings.SignatureMatch.Enabled {
		t.Fatalf("settings = %+v, err = %v", settings, err)
	}
}

func TestSettingsFromCatalogRejectsUnenforceableCatalogs(t *testing.T) {
	feed := feedJSON("feed_v1", sampleRule)
	replace := func(old, new string) string { return strings.Replace(samplePolicyContent, old, new, 1) }
	cases := map[string]struct {
		content string
		feed    string
		want    error
	}{
		"enabled signatures without feed": {samplePolicyContent, "", ErrFeedDigest},
		"feed revision differs":           {replace(`"revision":"feed_v1"`, `"revision":"feed_v2"`), feed, ErrFeed},
		"disabled rule not in feed":       {replace(`"disabled_rules":[]`, `"disabled_rules":["no_such_rule_v1"]`), feed, ErrSettings},
		"duplicate disabled rule": {replace(`"disabled_rules":[]`,
			`"disabled_rules":["prompt_ignore_previous_v1","prompt_ignore_previous_v1"]`), feed, ErrSettings},
		"unknown guard":                {replace(`"controls":{`, `"controls":{"custom_plugin":{"enabled":true,"boundaries":["tool_result"]},`), feed, ErrSettings},
		"missing guard":                {replace(`"signature_match":{"enabled":true,"boundaries":["model_input","tool_result","action_proposal"]}`, `"x":{}`), feed, ErrSettings},
		"unknown guard key":            {replace(`"mode":"redact",`, `"mode":"redact","script":"x",`), feed, ErrSettings},
		"mode on signature_match":      {replace(`"signature_match":{"enabled":true,`, `"signature_match":{"enabled":true,"mode":"block",`), feed, ErrSettings},
		"missing threshold":            {replace(`"threshold":0.75,`, ``), feed, ErrSettings},
		"threshold out of range":       {replace(`"threshold":0.75`, `"threshold":1.5`), feed, ErrSettings},
		"secret at action proposal":    {replace(`"boundaries":["model_input","tool_result"]`, `"boundaries":["action_proposal"]`), feed, ErrSettings},
		"empty boundaries":             {replace(`"boundaries":["model_input","tool_result"]`, `"boundaries":[]`), feed, ErrSettings},
		"mode allow":                   {replace(`"mode":"block"`, `"mode":"allow"`), feed, ErrSettings},
		"duplicate key":                {replace(`"threshold":0.75`, `"threshold":0.75,"threshold":0.1`), feed, ErrSettings},
		"missing signatures":           {replace(`"signatures":{"path":"attack-signatures.json","revision":"feed_v1","disabled_rules":[]},`, ``), feed, ErrSettings},
		"unknown signatures key":       {replace(`"disabled_rules":[]`, `"disabled_rules":[],"url":"https://example.com"`), feed, ErrSettings},
		"missing enabled":              {replace(`"secret_pattern":{"enabled":true,`, `"secret_pattern":{`), feed, ErrSettings},
		"feed bytes altered after pin": {samplePolicyContent, feed + " ", ErrFeedDigest},
	}
	for name, testCase := range cases {
		digest := digestOf(feed)
		if testCase.feed == "" {
			digest = ""
		}
		if _, err := SettingsFromCatalog(3, []byte(testCase.content), []byte(testCase.feed), digest); !errors.Is(err, testCase.want) {
			t.Fatalf("%s: err = %v, want %v", name, err, testCase.want)
		}
	}
	if _, err := SettingsFromCatalog(0, []byte(samplePolicyContent), []byte(feed), digestOf(feed)); !errors.Is(err, ErrSettings) {
		t.Fatalf("revision 0 accepted: %v", err)
	}
}
