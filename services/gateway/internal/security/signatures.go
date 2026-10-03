package security

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The feed grammar (lead's delegate, 3 October 2026): data-only rules whose pattern is a plain
// substring of the normalized text. No regular expression, code, URL or loading path is accepted.
const (
	maxFeedBytes       = 64 << 10
	maxFeedRules       = 100
	maxPatternBytes    = 256
	minPatternBytes    = 3
	maxFeedTextBytes   = 1000
	feedSchemaVersion  = 1
	patternTypeLiteral = "normalized_substring"
)

var (
	ErrFeed           = errors.New("invalid signature feed")
	ErrFeedDigest     = errors.New("signature feed digest mismatch")
	feedIdentifier    = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)
	feedRuleID        = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,63}$`)
	feedDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// SignatureRule is one validated feed rule.
type SignatureRule struct {
	ID          string
	AttackClass string
	Pattern     string
	Boundaries  []Boundary
	Response    Mode
}

// Feed is a validated signature feed, pinned to the SHA-256 digest of its file bytes.
type Feed struct {
	Issuer   string
	Revision string
	Digest   string
	Rules    []SignatureRule
}

// RuleIDs lists the feed's rule IDs in feed order.
func (feed *Feed) RuleIDs() []string {
	identifiers := make([]string, 0, len(feed.Rules))
	for _, rule := range feed.Rules {
		identifiers = append(identifiers, rule.ID)
	}
	return identifiers
}

type feedFile struct {
	SchemaVersion int        `json:"schema_version"`
	Issuer        string     `json:"issuer"`
	Revision      string     `json:"revision"`
	Description   string     `json:"description"`
	Scope         string     `json:"scope"`
	Rules         []feedRule `json:"rules"`
}

type feedRule struct {
	ID          string     `json:"id"`
	AttackClass string     `json:"attack_class"`
	Description string     `json:"description"`
	PatternType string     `json:"pattern_type"`
	Pattern     string     `json:"pattern"`
	Boundaries  []Boundary `json:"boundaries"`
	Response    Mode       `json:"response"`
	Sources     []string   `json:"sources"`
}

// ParseFeed validates the feed file bytes against the digest pinned by the active catalog. The
// digest proves the bytes are the ones the authenticated import accepted; it does not by itself
// authenticate the publisher. Any problem rejects the whole feed.
func ParseFeed(raw []byte, pinnedDigest string) (*Feed, error) {
	if !feedDigestPattern.MatchString(pinnedDigest) {
		return nil, ErrFeedDigest
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != pinnedDigest {
		return nil, ErrFeedDigest
	}
	if len(raw) > maxFeedBytes || !utf8.Valid(raw) || !uniqueJSONKeys(raw) {
		return nil, ErrFeed
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var file feedFile
	if decoder.Decode(&file) != nil {
		return nil, ErrFeed
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrFeed
	}
	if file.SchemaVersion != feedSchemaVersion || !feedIdentifier.MatchString(file.Issuer) || !feedIdentifier.MatchString(file.Revision) ||
		!boundedText(file.Description) || !boundedText(file.Scope) || len(file.Rules) == 0 || len(file.Rules) > maxFeedRules {
		return nil, ErrFeed
	}
	feed := &Feed{Issuer: file.Issuer, Revision: file.Revision, Digest: pinnedDigest}
	for _, rule := range file.Rules {
		if !validFeedRule(rule) || slices.Contains(feed.RuleIDs(), rule.ID) {
			return nil, ErrFeed
		}
		feed.Rules = append(feed.Rules, SignatureRule{
			ID: rule.ID, AttackClass: rule.AttackClass, Pattern: rule.Pattern,
			Boundaries: slices.Clone(rule.Boundaries), Response: rule.Response,
		})
	}
	return feed, nil
}

func validFeedRule(rule feedRule) bool {
	if !feedRuleID.MatchString(rule.ID) || !feedRuleID.MatchString(rule.AttackClass) || !boundedText(rule.Description) ||
		rule.PatternType != patternTypeLiteral || rule.Response != ModeBlock || len(rule.Sources) == 0 {
		return false
	}
	// A pattern must already be in normalized form, so the file shows exactly what is matched.
	if len(rule.Pattern) < minPatternBytes || len(rule.Pattern) > maxPatternBytes || NormalizeText(rule.Pattern) != rule.Pattern {
		return false
	}
	if len(rule.Boundaries) == 0 {
		return false
	}
	for index, boundary := range rule.Boundaries {
		if !slices.Contains([]Boundary{BoundaryModelInput, BoundaryToolResult, BoundaryActionProposal}, boundary) ||
			slices.Contains(rule.Boundaries[:index], boundary) {
			return false
		}
	}
	for _, source := range rule.Sources {
		if !boundedText(source) {
			return false
		}
	}
	return true
}

// boundedText accepts non-empty printable text of bounded length.
func boundedText(text string) bool {
	return text != "" && len(text) <= maxFeedTextBytes && strings.IndexFunc(text, unicode.IsControl) < 0
}

// NormalizeText lowercases text, drops invisible format characters (for example zero-width
// spaces) and collapses every whitespace run to one space, so trivial spacing or case changes
// do not evade a rule. It is not a defence against paraphrase or encoding.
func NormalizeText(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	pendingSpace := false
	for _, character := range text {
		switch {
		case unicode.Is(unicode.Cf, character):
			continue
		case unicode.IsSpace(character):
			pendingSpace = builder.Len() > 0
			continue
		}
		if pendingSpace {
			builder.WriteByte(' ')
			pendingSpace = false
		}
		builder.WriteRune(unicode.ToLower(character))
	}
	return builder.String()
}

// MatchSignatures checks one text against the active feed's rules for the boundary, skipping
// rules disabled in the catalog. The first matching rule in feed order blocks; the record names
// the rule, feed revision and digest. Text is never stored in the record.
func MatchSignatures(text string, boundary Boundary, field FieldName, settings Settings) (ControlRecord, error) {
	started := time.Now()
	record := ControlRecord{
		Boundary:                   boundary,
		Field:                      field,
		ControlClass:               ClassDeterministic,
		ControlID:                  ControlSignatureMatch,
		EvaluatedCatalogRevisionID: settings.EvaluatedCatalogRevisionID,
	}
	finish := func(outcome Outcome, reason, ruleID string, err error) (ControlRecord, error) {
		record.Outcome, record.ReasonCode, record.MatchedRuleID = outcome, reason, ruleID
		record.Duration = time.Since(started)
		return record, err
	}
	if err := settings.validate(); err != nil {
		return finish(OutcomeError, "", "", err)
	}
	if !settings.SignatureMatch.appliesAt(boundary) {
		return finish(OutcomeNotApplicable, "", "", nil)
	}
	if len(text) > MaxFieldBytes || !utf8.ValidString(text) {
		record.ControlID = ControlFieldLimit
		return finish(OutcomeBlock, ReasonContentTooLarge, "", nil)
	}
	record.FeedRevision, record.FeedDigest = settings.Feed.Revision, settings.Feed.Digest
	normalized := NormalizeText(text)
	for _, rule := range settings.Feed.Rules {
		if slices.Contains(settings.DisabledRules, rule.ID) || !slices.Contains(rule.Boundaries, boundary) {
			continue
		}
		if strings.Contains(normalized, rule.Pattern) {
			return finish(OutcomeBlock, ReasonSignatureMatch, rule.ID, nil)
		}
	}
	return finish(OutcomePass, "", "", nil)
}

// uniqueJSONKeys reports whether raw is one JSON value with no duplicate key in any object;
// encoding/json would otherwise keep the last duplicate silently.
func uniqueJSONKeys(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if !uniqueJSONValue(decoder, 0) {
		return false
	}
	_, err := decoder.Token()
	return err == io.EOF
}

func uniqueJSONValue(decoder *json.Decoder, depth int) bool {
	if depth > 32 {
		return false
	}
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return true
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, isString := keyToken.(string)
			if err != nil || !isString || seen[key] {
				return false
			}
			seen[key] = true
			if !uniqueJSONValue(decoder, depth+1) {
				return false
			}
		}
	case '[':
		for decoder.More() {
			if !uniqueJSONValue(decoder, depth+1) {
				return false
			}
		}
	default:
		return false
	}
	closing, err := decoder.Token()
	return err == nil && (closing == json.Delim('}') || closing == json.Delim(']'))
}
