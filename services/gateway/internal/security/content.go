package security

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// SecretKind is the kind of value a content rule detects; it appears in the mask text.
type SecretKind string

const (
	SecretPassword    SecretKind = "password"
	SecretAPIToken    SecretKind = "api_token"
	SecretBankAccount SecretKind = "bank_account"
	SecretPaymentCard SecretKind = "payment_card"
)

// Span is one detected secret: byte offsets into the field text, start inclusive, end exclusive.
type Span struct {
	Start  int
	End    int
	Kind   SecretKind
	RuleID string
}

// contentRule is one built-in secret pattern. The capture group marks the secret value, and
// accept rejects candidates that only look like secrets (checksums, credential shape).
type contentRule struct {
	id      string
	kind    SecretKind
	pattern *regexp.Regexp
	accept  func(value string) bool
}

// The keyword rules start after the beginning of the text or any character that is not a letter or
// digit, so DB_PASSWORD=... and access_token=... match while a keyword inside a longer word
// (adminPassword) does not. An optional quote after the keyword and before the value lets JSON and
// quoted assignments ("password": "...") match, masking only the value.
// The rule set covers the secret kinds of fixtures/semantic-corpus.json. Go's regexp is RE2,
// so matching time is linear in the input; the patterns are fixed code, not catalog data.
var contentRules = []contentRule{
	{
		id:      "secret_password_keyword_v1",
		kind:    SecretPassword,
		pattern: regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:password|passwd|passcode|pwd)\b["']?\s*(?:[:=]|is\b)?\s*["']?([^\s,;]+)`),
		accept:  credentialShape(8),
	},
	{
		id:      "secret_url_credential_v1",
		kind:    SecretPassword,
		pattern: regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.\-]*://[^\s:/@]+:([^\s/@]+)@`),
		accept:  func(value string) bool { return value != "" },
	},
	{
		id:      "secret_api_token_keyword_v1",
		kind:    SecretAPIToken,
		pattern: regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:token|api[_-]?key|secret|bearer)\b["']?\s*(?:[:=]|is\b)?\s*["']?([^\s,;]+)`),
		accept:  credentialShape(16),
	},
	{
		id:      "secret_iban_v1",
		kind:    SecretBankAccount,
		pattern: regexp.MustCompile(`(?i)\b([A-Z]{2}[0-9]{2}(?: ?[A-Z0-9]{4}){2,7}(?: ?[A-Z0-9]{1,3})?)\b`),
		accept:  validIBAN,
	},
	{
		id:      "secret_payment_card_v1",
		kind:    SecretPaymentCard,
		pattern: regexp.MustCompile(`\b([0-9](?:[ \-]?[0-9]){12,18})\b`),
		accept:  validCardNumber,
	},
}

// trailingPunctuation is stripped from a keyword-anchored value, so "password x1y2z3w4." masks
// only the value and not the sentence's full stop.
const trailingPunctuation = ".)]}'\"!?:"

// FindSecrets returns the validated, merged secret spans of text in ascending order.
func FindSecrets(text string) ([]Span, error) {
	var spans []Span
	for _, rule := range contentRules {
		for _, match := range rule.pattern.FindAllStringSubmatchIndex(text, -1) {
			start, end := match[2], match[3]
			for end > start && strings.ContainsRune(trailingPunctuation, rune(text[end-1])) {
				end--
			}
			if end > start && rule.accept(text[start:end]) {
				spans = append(spans, Span{Start: start, End: end, Kind: rule.kind, RuleID: rule.id})
			}
		}
	}
	return validateSpans(text, spans)
}

// validateSpans checks every span against the text and merges overlaps. A merged span keeps the
// kind and rule of the span that starts first (the longest on a tie). Any invalid span is an
// error, so a faulty rule can never cause a partial or misplaced mask.
func validateSpans(text string, spans []Span) ([]Span, error) {
	for _, span := range spans {
		if span.Start < 0 || span.End > len(text) || span.Start >= span.End ||
			!utf8.RuneStart(text[span.Start]) || (span.End < len(text) && !utf8.RuneStart(text[span.End])) {
			return nil, ErrSpan
		}
	}
	sorted := slices.Clone(spans)
	slices.SortFunc(sorted, func(left, right Span) int {
		if left.Start != right.Start {
			return left.Start - right.Start
		}
		return right.End - left.End
	})
	var merged []Span
	for _, span := range sorted {
		if last := len(merged) - 1; last >= 0 && span.Start < merged[last].End {
			merged[last].End = max(merged[last].End, span.End)
			continue
		}
		merged = append(merged, span)
	}
	return merged, nil
}

// MaskText replaces each validated span with a fixed mask naming its kind, for example
// "[REDACTED:password]". The mask text is deterministic so evidence can show the exact result.
func MaskText(text string, spans []Span) (string, error) {
	validated, err := validateSpans(text, spans)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	position := 0
	for _, span := range validated {
		builder.WriteString(text[position:span.Start])
		builder.WriteString("[REDACTED:" + string(span.Kind) + "]")
		position = span.End
	}
	builder.WriteString(text[position:])
	return builder.String(), nil
}

// ContentResult is the outcome of the deterministic content controls for one field. Text holds
// only content the controls permit: the original when it passed, the masked text when redacted,
// and nothing when the field is withheld. Source is the caller's provenance, unchanged.
type ContentResult struct {
	Field  FieldName
	Source SourceRef
	Text   string
	Spans  []Span
	Record ControlRecord
}

// Permitted reports whether the field may continue to the next control.
func (result ContentResult) Permitted() bool {
	switch result.Record.Outcome {
	case OutcomePass, OutcomeRedact, OutcomeNotApplicable:
		return true
	default:
		return false
	}
}

// ApplyContentRules applies the field limit and the configured secret patterns to one designated
// field. Every failure withholds the field: an error never lets unchecked text through.
func ApplyContentRules(field Field, boundary Boundary, settings Settings) (ContentResult, error) {
	started := time.Now()
	result := ContentResult{Field: field.Name, Source: field.Source}
	result.Record = ControlRecord{
		Boundary:                   boundary,
		Field:                      field.Name,
		ControlClass:               ClassDeterministic,
		ControlID:                  ControlSecretPattern,
		EvaluatedCatalogRevisionID: settings.EvaluatedCatalogRevisionID,
	}
	finish := func(outcome Outcome, reason, ruleID string, err error) (ContentResult, error) {
		result.Record.Outcome, result.Record.ReasonCode, result.Record.MatchedRuleID = outcome, reason, ruleID
		result.Record.Duration = time.Since(started)
		if !result.Permitted() {
			result.Text, result.Spans = "", nil
		}
		return result, err
	}

	if err := settings.validate(); err != nil {
		return finish(OutcomeError, "", "", err)
	}
	if !slices.Contains(designatedFields[boundary], field.Name) {
		return finish(OutcomeError, "", "", ErrField)
	}
	if len(field.Text) > MaxFieldBytes || !utf8.ValidString(field.Text) {
		result.Record.ControlID = ControlFieldLimit
		return finish(OutcomeBlock, ReasonContentTooLarge, "", nil)
	}
	result.Text = field.Text
	if !settings.SecretPattern.appliesAt(boundary) {
		return finish(OutcomeNotApplicable, "", "", nil)
	}

	// Invisible format characters (zero-width spaces, soft hyphens) must not split a keyword or a
	// value, so the rules read a copy without them. The spans are mapped back onto the original
	// text, so a masked field changes only where a secret was and keeps every other character,
	// including the joiners of emoji and the marks of right-to-left scripts.
	scanned, offsets := stripFormatCharacters(field.Text)
	spans, err := FindSecrets(scanned)
	if err != nil {
		return finish(OutcomeError, "", "", err)
	}
	if len(spans) == 0 {
		return finish(OutcomePass, "", "", nil)
	}
	if settings.SecretPattern.Mode == ModeBlock {
		return finish(OutcomeBlock, ReasonContentBlocked, spans[0].RuleID, nil)
	}
	spans = mapSpans(spans, offsets)
	masked, err := MaskText(field.Text, spans)
	if err != nil {
		return finish(OutcomeError, "", "", err)
	}
	result.Text, result.Spans = masked, spans
	return finish(OutcomeRedact, ReasonContentRedacted, spans[0].RuleID, nil)
}

// stripFormatCharacters drops the invisible format characters (Unicode category Cf) that
// NormalizeText also ignores for signature matching. offsets[i] is the byte offset in text of byte i
// of the result, so a span found in the result can be mapped back; the text must be valid UTF-8.
func stripFormatCharacters(text string) (stripped string, offsets []int) {
	var builder strings.Builder
	builder.Grow(len(text))
	offsets = make([]int, 0, len(text))
	for index, character := range text {
		if unicode.Is(unicode.Cf, character) {
			continue
		}
		width := utf8.RuneLen(character)
		builder.WriteRune(character)
		for step := range width {
			offsets = append(offsets, index+step)
		}
	}
	return builder.String(), offsets
}

// mapSpans moves spans found in the stripped copy onto the original text. A span starts at its
// first character and ends after its last one, so format characters inside it are masked with it
// and the ones just outside it are kept.
func mapSpans(spans []Span, offsets []int) []Span {
	mapped := make([]Span, 0, len(spans))
	for _, span := range spans {
		span.Start, span.End = offsets[span.Start], offsets[span.End-1]+1
		mapped = append(mapped, span)
	}
	return mapped
}

// credentialShape accepts a value of at least minimumLength that mixes letters and digits, so
// ordinary words after a keyword ("password policy", "token of thanks") are not masked.
func credentialShape(minimumLength int) func(string) bool {
	return func(value string) bool {
		if len(value) < minimumLength {
			return false
		}
		hasLetter := strings.IndexFunc(value, unicode.IsLetter) >= 0
		hasDigit := strings.IndexFunc(value, unicode.IsDigit) >= 0
		return hasLetter && hasDigit
	}
}

// validIBAN checks the ISO 13616 length and mod-97 checksum, ignoring spaces and letter case.
func validIBAN(value string) bool {
	compact := strings.ToUpper(strings.ReplaceAll(value, " ", ""))
	if len(compact) < 15 || len(compact) > 34 {
		return false
	}
	rearranged := compact[4:] + compact[:4]
	remainder := 0
	for _, character := range rearranged {
		switch {
		case character >= '0' && character <= '9':
			remainder = (remainder*10 + int(character-'0')) % 97
		case character >= 'A' && character <= 'Z':
			remainder = (remainder*100 + int(character-'A') + 10) % 97
		default:
			return false
		}
	}
	return remainder == 1
}

// validCardNumber checks the digit count and the Luhn checksum, ignoring spaces and hyphens.
func validCardNumber(value string) bool {
	digits := strings.NewReplacer(" ", "", "-", "").Replace(value)
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	for index := range len(digits) {
		digit := int(digits[len(digits)-1-index] - '0')
		if index%2 == 1 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}
