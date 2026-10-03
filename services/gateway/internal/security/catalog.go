package security

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
)

// The catalog content keys this package reads, as documented in config/README.md. The other
// top-level keys (models, budgets, reports) belong to other readers and are ignored here.
type catalogContent struct {
	Controls   *catalogControls   `json:"controls"`
	Signatures *catalogSignatures `json:"signatures"`
}

type catalogControls struct {
	SecretPattern     *catalogGuard `json:"secret_pattern"`
	SemanticInjection *catalogGuard `json:"semantic_injection"`
	SignatureMatch    *catalogGuard `json:"signature_match"`
}

type catalogGuard struct {
	Enabled    *bool       `json:"enabled"`
	Mode       *Mode       `json:"mode"`
	Threshold  *float64    `json:"threshold"`
	Boundaries *[]Boundary `json:"boundaries"`
}

type catalogSignatures struct {
	Path          *string   `json:"path"`
	Revision      *string   `json:"revision"`
	DisabledRules *[]string `json:"disabled_rules"`
}

// SettingsFromCatalog builds the security settings from one active catalog revision: its id, its
// content JSON, and the bytes and pinned SHA-256 digest of the feed on the active pointer
// (app.signature_feed_revisions source_text and file_digest). It rejects anything the controls
// could not enforce, so a broken catalog stops evaluation instead of weakening it. feedContent
// may be empty only when signature_match is disabled.
func SettingsFromCatalog(revisionID int64, content []byte, feedContent []byte, feedDigest string) (Settings, error) {
	if revisionID <= 0 || !uniqueJSONKeys(content) {
		return Settings{}, ErrSettings
	}
	var root catalogContent
	if json.Unmarshal(content, &root) != nil || root.Controls == nil || root.Signatures == nil {
		return Settings{}, ErrSettings
	}
	if !strictObject(content, "controls", "secret_pattern", "semantic_injection", "signature_match") {
		return Settings{}, ErrSettings
	}
	settings := Settings{EvaluatedCatalogRevisionID: revisionID}
	var err error
	if settings.SecretPattern, err = guardFromCatalog(root.Controls.SecretPattern, true, false,
		BoundaryModelInput, BoundaryToolResult); err != nil {
		return Settings{}, err
	}
	semantic := root.Controls.SemanticInjection
	if settings.SemanticInjection.GuardSettings, err = guardFromCatalog(semantic, true, true,
		BoundaryModelInput, BoundaryToolResult, BoundaryActionProposal); err != nil {
		return Settings{}, err
	}
	settings.SemanticInjection.Threshold = *semantic.Threshold
	if settings.SignatureMatch, err = guardFromCatalog(root.Controls.SignatureMatch, false, false,
		BoundaryModelInput, BoundaryToolResult, BoundaryActionProposal); err != nil {
		return Settings{}, err
	}

	signatures := root.Signatures
	if signatures.Path == nil || signatures.Revision == nil || signatures.DisabledRules == nil {
		return Settings{}, ErrSettings
	}
	settings.DisabledRules = slices.Clone(*signatures.DisabledRules)
	for index, ruleID := range settings.DisabledRules {
		if !feedRuleID.MatchString(ruleID) || slices.Contains(settings.DisabledRules[:index], ruleID) {
			return Settings{}, ErrSettings
		}
	}
	if len(feedContent) > 0 || settings.SignatureMatch.Enabled {
		feed, err := ParseFeed(feedContent, feedDigest)
		if err != nil {
			return Settings{}, err
		}
		// The policy names the feed revision it was written for; a different feed is not it.
		if feed.Revision != *signatures.Revision {
			return Settings{}, ErrFeed
		}
		for _, ruleID := range settings.DisabledRules {
			if !slices.Contains(feed.RuleIDs(), ruleID) {
				return Settings{}, ErrSettings
			}
		}
		settings.Feed = feed
	}
	if err := settings.validate(); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

// guardFromCatalog checks one guard's keys: all present, nothing unknown, supported boundaries.
func guardFromCatalog(guard *catalogGuard, hasMode, hasThreshold bool, supported ...Boundary) (GuardSettings, error) {
	if guard == nil || guard.Enabled == nil || guard.Boundaries == nil || (guard.Mode != nil) != hasMode || (guard.Threshold != nil) != hasThreshold {
		return GuardSettings{}, ErrSettings
	}
	settings := GuardSettings{Enabled: *guard.Enabled, Boundaries: slices.Clone(*guard.Boundaries)}
	if hasMode {
		settings.Mode = *guard.Mode
		if settings.Mode != ModeBlock && settings.Mode != ModeRedact {
			return GuardSettings{}, ErrSettings
		}
	}
	if hasThreshold && (math.IsNaN(*guard.Threshold) || *guard.Threshold < 0 || *guard.Threshold > 1) {
		return GuardSettings{}, ErrSettings
	}
	if len(settings.Boundaries) == 0 {
		return GuardSettings{}, ErrSettings
	}
	for index, boundary := range settings.Boundaries {
		if !slices.Contains(supported, boundary) || slices.Contains(settings.Boundaries[:index], boundary) {
			return GuardSettings{}, ErrSettings
		}
	}
	return settings, nil
}

// strictObject rejects unknown keys inside the controls object and inside each guard, so a
// misspelled or invented setting is never silently ignored.
func strictObject(content []byte, key string, guards ...string) bool {
	var root map[string]json.RawMessage
	if json.Unmarshal(content, &root) != nil {
		return false
	}
	var controls map[string]json.RawMessage
	if json.Unmarshal(root[key], &controls) != nil || len(controls) != len(guards) {
		return false
	}
	for _, guard := range guards {
		raw, present := controls[guard]
		if !present {
			return false
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&catalogGuard{}) != nil {
			return false
		}
	}
	var signatures map[string]json.RawMessage
	return json.Unmarshal(root["signatures"], &signatures) == nil && len(signatures) == 3
}
