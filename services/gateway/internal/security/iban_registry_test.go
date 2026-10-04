package security

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// The IBAN rule needs a registered country, that country's exact length and a valid mod-97
// checksum, so an identifier that only happens to look like one is not masked.
func TestValidIBANNeedsARegisteredCountryAndItsLength(t *testing.T) {
	registered := []string{
		"PL61109010140000071219812874", "GB82WEST12345698765432", "DE89370400440532013000",
		"FR1420041010050500013M02606", "NL91ABNA0417164300", "ES9121000418450200051332",
		"IT60X0542811101000000123456", "BE68539007547034", "NO9386011117947", "CH9300762011623852957",
		"AT611904300234573201",
	}
	for _, iban := range registered {
		for _, variant := range []string{iban, strings.ToLower(iban), iban[:4] + " " + iban[4:]} {
			if !validIBAN(variant) {
				t.Errorf("registered IBAN %q was refused", variant)
			}
		}
	}
	// Valid mod-97 checksums that the registry still refuses: a known country at the wrong length,
	// and countries that are not in the registry.
	notIBANs := map[string]string{
		"DE too short":    "DE9452601815908301661",
		"DE too long":     "DE5431860913909960308246",
		"GB too short":    "GB18WEST2819482199351",
		"GB too long":     "GB66WEST819093786579754",
		"NO too long":     "NO10323194875749",
		"unknown ZZ":      "ZZ92118625276018955597",
		"unknown AB":      "AB7997114710497465075291",
		"too short to be": "DE8",
	}
	for name, value := range notIBANs {
		if validIBAN(value) {
			t.Errorf("%s: %q was taken for an IBAN", name, value)
		}
	}
}

// Lower-case hex identifiers (a MongoDB ObjectId is 24 characters) were flagged about once in 1,400
// when the pattern became case insensitive. With the registry they are not, for a fixed sample of 40,000.
func TestHexIdentifiersAreNotTakenForIBANs(t *testing.T) {
	random := rand.New(rand.NewSource(20261004))
	for range 40000 {
		length := 24 + random.Intn(5)
		identifier := make([]byte, length)
		for index := range identifier {
			identifier[index] = "0123456789abcdef"[random.Intn(16)]
		}
		text := "object id " + string(identifier) + " created"
		spans, err := FindSecrets(text)
		if err != nil {
			t.Fatal(err)
		}
		for _, span := range spans {
			if span.Kind == SecretBankAccount {
				t.Fatalf("hex identifier taken for an IBAN: %s", fmt.Sprintf("%q", text))
			}
		}
	}
}

// validIBANFor builds an IBAN of the registered length for a country: a deterministic digit BBAN and
// the mod-97 check digits that make it valid.
func validIBANFor(country string, length int) string {
	bban := make([]byte, length-4)
	for index := range bban {
		bban[index] = "1234567890"[(index*7+3)%10]
	}
	remainder := 0
	for _, character := range string(bban) + country + "00" {
		if character >= '0' && character <= '9' {
			remainder = (remainder*10 + int(character-'0')) % 97
		} else {
			remainder = (remainder*100 + int(character-'A') + 10) % 97
		}
	}
	return fmt.Sprintf("%s%02d%s", country, 98-remainder, bban)
}

// groupedInFours writes an IBAN the way it is printed: groups of four separated by single spaces.
func groupedInFours(iban string) string {
	var groups []string
	for len(iban) > 4 {
		groups, iban = append(groups, iban[:4]), iban[4:]
	}
	return strings.Join(append(groups, iban), " ")
}

// A trailing word must never be absorbed into the IBAN candidate: every registered country, compact
// and grouped, in either letter case, is masked exactly, whatever follows it.
func TestEveryRegisteredIBANIsMaskedExactlyWhateverFollows(t *testing.T) {
	suffixes := []string{"", ".", " ok", " for Atlas", " FOR ATLAS", " today", " tomorrow", ", thanks", "\nnext line"}
	prefixes := []string{"", "IBAN: ", "pay to "}
	for country, length := range ibanLengths {
		compact := validIBANFor(country, length)
		if !validIBAN(compact) {
			t.Fatalf("%s: the generated %q is not a valid IBAN", country, compact)
		}
		for _, written := range []string{compact, groupedInFours(compact), strings.ToLower(compact), strings.ToLower(groupedInFours(compact))} {
			for _, prefix := range prefixes {
				for _, suffix := range suffixes {
					text := prefix + written + suffix
					spans, err := FindSecrets(text)
					if err != nil {
						t.Fatal(err)
					}
					if len(spans) != 1 || spans[0].Kind != SecretBankAccount || text[spans[0].Start:spans[0].End] != written {
						t.Errorf("%q: spans %+v, want exactly the IBAN %q", text, spans, written)
						continue
					}
					if masked, _ := MaskText(text, spans); masked != prefix+"[REDACTED:bank_account]"+suffix {
						t.Errorf("%q: masked %q", text, masked)
					}
				}
			}
		}
	}
}
