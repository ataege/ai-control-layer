package security

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const fixturesDirectory = "../../../../fixtures"

type corpusSecret struct {
	Kind  SecretKind `json:"kind"`
	Value string     `json:"value"`
	Start int        `json:"start"`
	End   int        `json:"end"`
}

type corpusCase struct {
	ID              string         `json:"id"`
	Category        string         `json:"category"`
	Boundary        Boundary       `json:"boundary"`
	Text            string         `json:"text"`
	ExpectedOutcome string         `json:"expected_outcome"`
	Secrets         []corpusSecret `json:"secrets"`
	SignatureRule   string         `json:"signature_rule"`
}

func loadCorpus(t *testing.T) []corpusCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixturesDirectory, "semantic-corpus.json"))
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var corpus struct {
		Cases []corpusCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("decode corpus: %v", err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatal("corpus has no cases")
	}
	return corpus.Cases
}

func loadHostileNotes(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixturesDirectory, "hostile-notes.json"))
	if err != nil {
		t.Fatalf("read hostile notes: %v", err)
	}
	var notes struct {
		Notes []struct {
			Text string `json:"text"`
		} `json:"notes"`
	}
	if err := json.Unmarshal(raw, &notes); err != nil {
		t.Fatalf("decode hostile notes: %v", err)
	}
	texts := make([]string, 0, len(notes.Notes))
	for _, note := range notes.Notes {
		texts = append(texts, note.Text)
	}
	return texts
}

func redactSettings(mode Mode) Settings {
	return Settings{
		EvaluatedCatalogRevisionID: 7,
		SecretPattern:              GuardSettings{Enabled: true, Mode: mode, Boundaries: []Boundary{BoundaryModelInput, BoundaryToolResult}},
	}
}

func fieldFor(boundary Boundary, text string) Field {
	name := FieldToolResultText
	if boundary == BoundaryModelInput {
		name = FieldModelInputText
	}
	return Field{Name: name, Text: text, Source: SourceRef{SourceID: "invoice_A01", Version: 1, Classification: "internal_only"}}
}

// Every secret case yields exactly its fixture spans; no other case yields any span, which covers
// the benign hard negatives ("password policy", "ignore the earlier invoice") and the attacks.
func TestFindSecretsMatchesCorpusSpans(t *testing.T) {
	for _, testCase := range loadCorpus(t) {
		t.Run(testCase.ID, func(t *testing.T) {
			spans, err := FindSecrets(testCase.Text)
			if err != nil {
				t.Fatalf("FindSecrets: %v", err)
			}
			var expected []Span
			for _, secret := range testCase.Secrets {
				if testCase.Text[secret.Start:secret.End] != secret.Value {
					t.Fatalf("fixture span does not hold its value")
				}
				expected = append(expected, Span{Start: secret.Start, End: secret.End, Kind: secret.Kind})
			}
			var found []Span
			for _, span := range spans {
				found = append(found, Span{Start: span.Start, End: span.End, Kind: span.Kind})
			}
			if !slices.Equal(found, expected) {
				t.Fatalf("spans = %+v, want %+v", found, expected)
			}
		})
	}
}

func TestFindSecretsIgnoresHostileNotes(t *testing.T) {
	for _, text := range loadHostileNotes(t) {
		if spans, err := FindSecrets(text); err != nil || len(spans) != 0 {
			t.Fatalf("hostile note spans = %+v, err = %v", spans, err)
		}
	}
}

func TestEachRuleDetectsItsKind(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		ruleID string
		value  string
	}{
		{"password keyword", "the pwd: Zx9qWm2pLk is new", "secret_password_keyword_v1", "Zx9qWm2pLk"},
		{"password trailing stop", "Set password=k3yB0ardCat.", "secret_password_keyword_v1", "k3yB0ardCat"},
		{"url credential", "dsn mysql://reader:s3cr3t@db.example/x", "secret_url_credential_v1", "s3cr3t"},
		{"api key", "api_key=ab12cd34ef56gh78ij90", "secret_api_token_keyword_v1", "ab12cd34ef56gh78ij90"},
		{"iban without spaces", "pay to DE89370400440532013000 today", "secret_iban_v1", "DE89370400440532013000"},
		{"card with hyphens", "card 5555-5555-5555-4444 on file", "secret_payment_card_v1", "5555-5555-5555-4444"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			spans, err := FindSecrets(testCase.text)
			if err != nil || len(spans) != 1 {
				t.Fatalf("spans = %+v, err = %v", spans, err)
			}
			if spans[0].RuleID != testCase.ruleID || testCase.text[spans[0].Start:spans[0].End] != testCase.value {
				t.Fatalf("span = %+v (%q)", spans[0], testCase.text[spans[0].Start:spans[0].End])
			}
		})
	}
}

// Checksums and credential shape keep look-alike values unmasked.
func TestFindSecretsRejectsLookAlikes(t *testing.T) {
	for _, text := range []string{
		"card 4111 1111 1111 1112 fails Luhn",
		"IBAN GB83 WEST 1234 5698 7654 32 fails mod-97",
		"password reset requested; password policy unchanged",
		"token of thanks, api_key rotation planned",
		"order 2026-10-31 and reference INV104, total 1250.00 EUR",
	} {
		if spans, err := FindSecrets(text); err != nil || len(spans) != 0 {
			t.Fatalf("%q: spans = %+v, err = %v", text, spans, err)
		}
	}
}

func TestValidateSpansRejectsInvalidSpans(t *testing.T) {
	text := "aé b"
	for _, spans := range [][]Span{
		{{Start: -1, End: 2}},
		{{Start: 2, End: 9}},
		{{Start: 3, End: 3}},
		{{Start: 2, End: 3}}, // starts inside the two-byte rune
		{{Start: 0, End: 2}}, // ends inside the two-byte rune
	} {
		if _, err := validateSpans(text, spans); !errors.Is(err, ErrSpan) {
			t.Fatalf("spans %+v: err = %v, want ErrSpan", spans, err)
		}
	}
}

func TestValidateSpansMergesOverlaps(t *testing.T) {
	text := "0123456789"
	merged, err := validateSpans(text, []Span{
		{Start: 6, End: 8, Kind: SecretAPIToken},
		{Start: 1, End: 4, Kind: SecretPassword},
		{Start: 1, End: 5, Kind: SecretBankAccount},
		{Start: 3, End: 6, Kind: SecretPaymentCard},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []Span{{Start: 1, End: 6, Kind: SecretBankAccount}, {Start: 6, End: 8, Kind: SecretAPIToken}}
	if !slices.Equal(merged, want) {
		t.Fatalf("merged = %+v, want %+v", merged, want)
	}
}

func TestRedactModeMasksExactSerialization(t *testing.T) {
	text := "Handover note: backup portal password demo-only-pass-8823 and recovery token demo-only-token-71c4e09ab2d35f86 are stored here until IT moves them."
	field := fieldFor(BoundaryToolResult, text)
	result, err := ApplyContentRules(field, BoundaryToolResult, redactSettings(ModeRedact))
	if err != nil {
		t.Fatal(err)
	}
	want := "Handover note: backup portal password [REDACTED:password] and recovery token [REDACTED:api_token] are stored here until IT moves them."
	if result.Text != want {
		t.Fatalf("text = %q", result.Text)
	}
	record := result.Record
	if record.Outcome != OutcomeRedact || record.ReasonCode != ReasonContentRedacted || record.MatchedRuleID != "secret_password_keyword_v1" ||
		record.ControlID != ControlSecretPattern || record.EvaluatedCatalogRevisionID != 7 || !result.Permitted() {
		t.Fatalf("record = %+v", record)
	}
	// Masking never changes the trusted source restriction.
	if result.Source != field.Source {
		t.Fatalf("source = %+v, want %+v", result.Source, field.Source)
	}
}

func TestBlockModeWithholdsField(t *testing.T) {
	field := fieldFor(BoundaryToolResult, "Vendor onboarding note: portal login is atlas-billing, password demo-only-pass-4471, rotate after first use.")
	result, err := ApplyContentRules(field, BoundaryToolResult, redactSettings(ModeBlock))
	if err != nil {
		t.Fatal(err)
	}
	if result.Permitted() || result.Text != "" || result.Spans != nil {
		t.Fatalf("blocked field leaked content: %+v", result)
	}
	if result.Record.Outcome != OutcomeBlock || result.Record.ReasonCode != ReasonContentBlocked || result.Record.MatchedRuleID != "secret_password_keyword_v1" {
		t.Fatalf("record = %+v", result.Record)
	}
	if result.Source != field.Source {
		t.Fatalf("source changed: %+v", result.Source)
	}
}

func TestModelInputBoundaryRedacts(t *testing.T) {
	text := "Use this connection to check the totals: postgres://report_reader:demo-only-db-pass-2026@db.finance.example/ledger"
	result, err := ApplyContentRules(fieldFor(BoundaryModelInput, text), BoundaryModelInput, redactSettings(ModeRedact))
	if err != nil || result.Record.Outcome != OutcomeRedact {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if strings.Contains(result.Text, "demo-only-db-pass-2026") || !strings.Contains(result.Text, "report_reader:[REDACTED:password]@") {
		t.Fatalf("text = %q", result.Text)
	}
}

func TestCleanFieldPasses(t *testing.T) {
	text := "Invoice invoice_A01 from Atlas, external reference INV104, total 1250.00 EUR, due 2026-10-31."
	result, err := ApplyContentRules(fieldFor(BoundaryToolResult, text), BoundaryToolResult, redactSettings(ModeRedact))
	if err != nil || result.Record.Outcome != OutcomePass || result.Text != text || result.Record.MatchedRuleID != "" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

// A disabled guard or an unconfigured boundary is an authorized configuration, recorded as such.
func TestGuardNotApplicable(t *testing.T) {
	text := "password demo-only-pass-4471"
	disabled := redactSettings(ModeRedact)
	disabled.SecretPattern.Enabled = false
	toolOnly := redactSettings(ModeRedact)
	toolOnly.SecretPattern.Boundaries = []Boundary{BoundaryToolResult}
	for name, check := range map[string]struct {
		settings Settings
		boundary Boundary
	}{
		"disabled":         {disabled, BoundaryToolResult},
		"other boundaries": {toolOnly, BoundaryModelInput},
	} {
		result, err := ApplyContentRules(fieldFor(check.boundary, text), check.boundary, check.settings)
		if err != nil || result.Record.Outcome != OutcomeNotApplicable || result.Text != text {
			t.Fatalf("%s: result = %+v, err = %v", name, result, err)
		}
	}
}

// Oversized and invalid UTF-8 fields are withheld whole, even when the guard is disabled.
func TestFieldLimitWithholdsWholeField(t *testing.T) {
	disabled := redactSettings(ModeRedact)
	disabled.SecretPattern.Enabled = false
	for name, text := range map[string]string{
		"oversized":    strings.Repeat("a", MaxFieldBytes+1),
		"invalid utf8": "note \xff\xfe",
	} {
		result, err := ApplyContentRules(fieldFor(BoundaryToolResult, text), BoundaryToolResult, disabled)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if result.Permitted() || result.Text != "" || result.Record.ControlID != ControlFieldLimit || result.Record.ReasonCode != ReasonContentTooLarge {
			t.Fatalf("%s: result = %+v", name, result)
		}
	}
	atLimit := strings.Repeat("a", MaxFieldBytes)
	if result, err := ApplyContentRules(fieldFor(BoundaryToolResult, atLimit), BoundaryToolResult, disabled); err != nil || !result.Permitted() {
		t.Fatalf("field at the limit: result outcome %q, err = %v", result.Record.Outcome, err)
	}
}

// Missing catalog settings, an unknown mode or an undesignated field fail closed with no text.
func TestInvalidInputFailsClosed(t *testing.T) {
	text := "password demo-only-pass-4471"
	noRevision := redactSettings(ModeRedact)
	noRevision.EvaluatedCatalogRevisionID = 0
	badMode := redactSettings("allow")
	cases := map[string]struct {
		field    Field
		boundary Boundary
		settings Settings
		want     error
	}{
		"no catalog revision": {fieldFor(BoundaryToolResult, text), BoundaryToolResult, noRevision, ErrSettings},
		"unknown mode":        {fieldFor(BoundaryToolResult, text), BoundaryToolResult, badMode, ErrSettings},
		"note at model input": {Field{Name: FieldInternalNote, Text: text}, BoundaryModelInput, redactSettings(ModeRedact), ErrField},
		"action proposal":     {fieldFor(BoundaryToolResult, text), BoundaryActionProposal, redactSettings(ModeRedact), ErrField},
	}
	for name, testCase := range cases {
		result, err := ApplyContentRules(testCase.field, testCase.boundary, testCase.settings)
		if !errors.Is(err, testCase.want) || result.Permitted() || result.Text != "" || result.Record.Outcome != OutcomeError {
			t.Fatalf("%s: result = %+v, err = %v", name, result, err)
		}
	}
}
