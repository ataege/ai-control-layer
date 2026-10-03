package operatorcontext

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/logging"
)

const testSigningKey = "operator-context-test-key-0123456789abcdef"

var testNow = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

var testOperator = contracts.OperatorContext{
	UserID:         "2c3d4e5f-6a7b-4c8d-9e0f-1a2b3c4d5e6f",
	OrganizationID: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
	Roles:          []string{"operator"},
}

// signForTest builds a token the way apps/api gateway-client.service.ts does (jose SignJWT).
func signForTest(t *testing.T, key string, header, claims map[string]any) string {
	t.Helper()
	encode := func(value any) string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(encoded)
	}
	signingInput := encode(header) + "." + encode(claims)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validHeader() map[string]any { return map[string]any{"alg": "HS256"} }

func validClaims(tokenID string) map[string]any {
	return map[string]any{
		"ctx": testOperator,
		"iat": testNow.Unix(),
		"exp": testNow.Add(time.Minute).Unix(),
		"aud": "gateway",
		"iss": "gateway-client",
		"jti": tokenID,
	}
}

func newTestVerifier(t *testing.T) *Verifier {
	t.Helper()
	verifier, err := NewVerifier(logging.NewSecret(testSigningKey))
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	verifier.now = func() time.Time { return testNow }
	return verifier
}

func TestVerifyAcceptsTheAPIsTokenOnce(t *testing.T) {
	verifier := newTestVerifier(t)
	token := signForTest(t, testSigningKey, validHeader(), validClaims("jti-1"))
	operator, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if operator.UserID != testOperator.UserID || operator.OrganizationID != testOperator.OrganizationID ||
		len(operator.Roles) != 1 || operator.Roles[0] != "operator" {
		t.Errorf("operator = %+v", operator)
	}
	if _, err := verifier.Verify(token); !errors.Is(err, ErrInvalid) {
		t.Errorf("a replayed token was accepted: %v", err)
	}
	// An audience list that includes the gateway is a valid RFC 7519 audience.
	listClaims := validClaims("jti-2")
	listClaims["aud"] = []string{"other", "gateway"}
	if _, err := verifier.Verify(signForTest(t, testSigningKey, validHeader(), listClaims)); err != nil {
		t.Errorf("audience list: %v", err)
	}
}

// invalidTokenCase builds one rejected token: a raw string, or a signed header and claims,
// optionally signed with another key or with its payload replaced after signing.
type invalidTokenCase struct {
	key           string
	header        map[string]any
	claims        map[string]any
	rawToken      string
	tamperPayload bool
}

func TestVerifyRejectsEveryInvalidToken(t *testing.T) {
	mutate := func(change func(header, claims map[string]any)) (map[string]any, map[string]any) {
		header, claims := validHeader(), validClaims("jti-mutated")
		change(header, claims)
		return header, claims
	}
	cases := map[string]invalidTokenCase{
		"wrong key":                     {key: "another-signing-key-0123456789abcdefgh"},
		"altered payload after signing": {tamperPayload: true},
		"alg none": {rawToken: base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." +
			base64.RawURLEncoding.EncodeToString([]byte(`{}`)) + "."},
		"not three segments": {rawToken: "abc.def"},
		"empty":              {rawToken: ""},
		"oversized":          {rawToken: strings.Repeat("a", 9000)},
	}
	add := func(name string, change func(header, claims map[string]any)) {
		header, claims := mutate(change)
		cases[name] = invalidTokenCase{header: header, claims: claims}
	}
	add("HS512 header", func(header, _ map[string]any) { header["alg"] = "HS512" })
	add("unknown header field", func(header, _ map[string]any) { header["kid"] = "x" })
	add("wrong issuer", func(_, claims map[string]any) { claims["iss"] = "someone-else" })
	add("audience of another service", func(_, claims map[string]any) { claims["aud"] = "api" })
	add("missing audience", func(_, claims map[string]any) { delete(claims, "aud") })
	add("expired", func(_, claims map[string]any) {
		claims["iat"] = testNow.Add(-2 * time.Minute).Unix()
		claims["exp"] = testNow.Add(-time.Minute).Unix()
	})
	add("issued in the future", func(_, claims map[string]any) {
		claims["iat"] = testNow.Add(time.Minute).Unix()
		claims["exp"] = testNow.Add(2 * time.Minute).Unix()
	})
	add("lifetime too long", func(_, claims map[string]any) { claims["exp"] = testNow.Add(time.Hour).Unix() })
	add("missing expiry", func(_, claims map[string]any) { delete(claims, "exp") })
	add("missing token id", func(_, claims map[string]any) { delete(claims, "jti") })
	add("unknown claim", func(_, claims map[string]any) { claims["admin"] = true })
	add("missing context", func(_, claims map[string]any) { delete(claims, "ctx") })
	add("organization not a uuid", func(_, claims map[string]any) {
		claims["ctx"] = map[string]any{"userId": testOperator.UserID, "organizationId": "demo_org", "roles": []string{}}
	})
	add("unknown context field", func(_, claims map[string]any) {
		claims["ctx"] = map[string]any{"userId": testOperator.UserID, "organizationId": testOperator.OrganizationID, "roles": []string{}, "admin": true}
	})
	add("missing roles", func(_, claims map[string]any) {
		claims["ctx"] = map[string]any{"userId": testOperator.UserID, "organizationId": testOperator.OrganizationID}
	})
	add("duplicate roles", func(_, claims map[string]any) {
		claims["ctx"] = map[string]any{"userId": testOperator.UserID, "organizationId": testOperator.OrganizationID, "roles": []string{"a", "a"}}
	})

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			verifier := newTestVerifier(t)
			token := testCase.rawToken
			mustSign := testCase.header != nil || testCase.key != "" || testCase.tamperPayload
			if mustSign {
				key := testSigningKey
				if testCase.key != "" {
					key = testCase.key
				}
				header, claims := testCase.header, testCase.claims
				if header == nil {
					header, claims = validHeader(), validClaims("jti-case")
				}
				token = signForTest(t, key, header, claims)
				if testCase.tamperPayload {
					forged := validClaims("jti-case")
					forged["ctx"] = map[string]any{"userId": testOperator.UserID,
						"organizationId": "11111111-2222-4333-8444-555555555555", "roles": []string{"operator"}}
					forgedPayload, _ := json.Marshal(forged)
					parts := strings.Split(token, ".")
					token = parts[0] + "." + base64.RawURLEncoding.EncodeToString(forgedPayload) + "." + parts[2]
				}
			}
			if _, err := verifier.Verify(token); !errors.Is(err, ErrInvalid) {
				t.Errorf("token accepted: %v", err)
			}
		})
	}
}

func TestNewVerifierRejectsShortKeysAndNilVerifierRejects(t *testing.T) {
	if _, err := NewVerifier(logging.NewSecret(strings.Repeat("k", MinimumKeyLength-1))); err == nil {
		t.Error("a short key was accepted")
	}
	var verifier *Verifier
	if _, err := verifier.Verify("a.b.c"); !errors.Is(err, ErrInvalid) {
		t.Errorf("nil verifier: %v", err)
	}
}

func TestReplayCacheRefusesWhenFullAndForgetsExpiredIDs(t *testing.T) {
	verifier := newTestVerifier(t)
	for index := 0; index < maximumRememberedTokens; index++ {
		verifier.usedTokenIDs["old-"+strconv.Itoa(index)] = testNow.Add(time.Minute)
	}
	if verifier.rememberTokenID("new", testNow.Add(time.Minute), testNow) {
		t.Error("a full cache accepted a new token id")
	}
	// Once the remembered ids expire they are pruned and new tokens are accepted again.
	if !verifier.rememberTokenID("new", testNow.Add(3*time.Minute), testNow.Add(2*time.Minute)) {
		t.Error("expired ids were not pruned")
	}
}

func TestContextRoundTrip(t *testing.T) {
	if _, ok := FromContext(t.Context()); ok {
		t.Error("an empty context carried an operator")
	}
	operator, ok := FromContext(WithOperator(t.Context(), testOperator))
	if !ok || operator.OrganizationID != testOperator.OrganizationID {
		t.Errorf("operator = %+v, ok = %v", operator, ok)
	}
}

// joseToken was signed by the API's own library (jose 6, SignJWT exactly as in
// apps/api gateway-client.service.ts) with testSigningKey at testNow, so this test fails if the
// Go verifier and the real signer ever disagree.
const joseToken = "eyJhbGciOiJIUzI1NiJ9.eyJjdHgiOnsidXNlcklkIjoiMmMzZDRlNWYtNmE3Yi00YzhkLTllMGYtMWEyYjNjNGQ1ZTZmIiwib3JnYW5pemF0aW9uSWQiOiIwYjlhM2MyZS01ZDRmLTRhNjEtOWI3ZS0zZjJkMWMwYTllMDEiLCJyb2xlcyI6WyJvcGVyYXRvciJdfSwiaWF0IjoxNzkxMDI4ODAwLCJhdWQiOiJnYXRld2F5IiwiZXhwIjoxNzkxMDI4ODYwLCJqdGkiOiJqb3NlLWZpeHR1cmUtanRpIiwiaXNzIjoiZ2F0ZXdheS1jbGllbnQifQ.qXuI04WV40LphMnQwn2czW2n8DSTU5dkhV9P0Fldu_0"

func TestVerifyAcceptsATokenSignedByTheAPILibrary(t *testing.T) {
	operator, err := newTestVerifier(t).Verify(joseToken)
	if err != nil {
		t.Fatalf("the API's token was rejected: %v", err)
	}
	if operator.OrganizationID != testOperator.OrganizationID || operator.UserID != testOperator.UserID {
		t.Errorf("operator = %+v", operator)
	}
}
