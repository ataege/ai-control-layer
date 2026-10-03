// Package operatorcontext verifies the signed operator context NestJS sends with every runtime
// command (decision 4): an HS256 JWT in the X-Operator-Context header, signed with
// OPERATOR_CONTEXT_SIGNING_KEY, issuer gateway-client, audience gateway, a short lifetime and a
// unique jti, carrying the X-14 OperatorContext as claim ctx. A valid token identifies the actor
// and organization; it never authorizes a command on its own.
package operatorcontext

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/logging"
)

// HeaderName carries the signed operator context on every internal command.
const HeaderName = "X-Operator-Context"

const (
	expectedIssuer   = "gateway-client"
	expectedAudience = "gateway"
	// MinimumKeyLength matches the API's rule for OPERATOR_CONTEXT_SIGNING_KEY.
	MinimumKeyLength = 32
	// maximumTokenBytes bounds parsing work on untrusted input.
	maximumTokenBytes = 8 << 10
	// maximumLifetime rejects a token whose issuer granted it more than a short life.
	maximumLifetime = 5 * time.Minute
	// clockLeeway tolerates small clock differences between the API and the gateway.
	clockLeeway = 5 * time.Second
	// maximumRememberedTokens bounds the replay cache; when full, new tokens are refused.
	maximumRememberedTokens = 100_000
	maximumJTILength        = 128
	maximumRoles            = 16
	maximumRoleLength       = 64
)

// ErrInvalid is the single error for every rejected token, so a caller learns nothing about
// which check failed.
var ErrInvalid = errors.New("missing or invalid operator context")

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Verifier checks operator-context tokens and remembers used token ids until they expire.
type Verifier struct {
	key          []byte
	now          func() time.Time
	mutex        sync.Mutex
	usedTokenIDs map[string]time.Time
}

// NewVerifier returns a verifier for the shared signing key.
func NewVerifier(signingKey logging.Secret) (*Verifier, error) {
	if signingKey.Len() < MinimumKeyLength {
		return nil, ErrInvalid
	}
	return &Verifier{
		key:          []byte(signingKey.Reveal()),
		now:          time.Now,
		usedTokenIDs: make(map[string]time.Time),
	}, nil
}

type tokenHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ,omitempty"`
}

type tokenClaims struct {
	Context   *contracts.OperatorContext `json:"ctx"`
	IssuedAt  *int64                     `json:"iat"`
	ExpiresAt *int64                     `json:"exp"`
	Audience  json.RawMessage            `json:"aud"`
	Issuer    string                     `json:"iss"`
	TokenID   string                     `json:"jti"`
}

// Verify returns the operator context of a valid, unused token. Each token is accepted once.
func (verifier *Verifier) Verify(token string) (contracts.OperatorContext, error) {
	if verifier == nil || len(token) == 0 || len(token) > maximumTokenBytes {
		return contracts.OperatorContext{}, ErrInvalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return contracts.OperatorContext{}, ErrInvalid
	}
	// The signature is checked before any claim is trusted.
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return contracts.OperatorContext{}, ErrInvalid
	}
	mac := hmac.New(sha256.New, verifier.key)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return contracts.OperatorContext{}, ErrInvalid
	}
	var header tokenHeader
	if decodeSegment(parts[0], &header) != nil || header.Algorithm != "HS256" ||
		(header.Type != "" && header.Type != "JWT") {
		return contracts.OperatorContext{}, ErrInvalid
	}
	var claims tokenClaims
	if decodeSegment(parts[1], &claims) != nil {
		return contracts.OperatorContext{}, ErrInvalid
	}
	now := verifier.now()
	expiresAt, valid := verifier.checkClaims(claims, now)
	if !valid {
		return contracts.OperatorContext{}, ErrInvalid
	}
	if !verifier.rememberTokenID(claims.TokenID, expiresAt, now) {
		return contracts.OperatorContext{}, ErrInvalid
	}
	return *claims.Context, nil
}

// checkClaims validates issuer, audience, lifetime, token id and the operator context.
func (verifier *Verifier) checkClaims(claims tokenClaims, now time.Time) (time.Time, bool) {
	if claims.Issuer != expectedIssuer || !audienceIncludesGateway(claims.Audience) ||
		claims.IssuedAt == nil || claims.ExpiresAt == nil || claims.Context == nil ||
		claims.TokenID == "" || len(claims.TokenID) > maximumJTILength || !utf8.ValidString(claims.TokenID) {
		return time.Time{}, false
	}
	issuedAt := time.Unix(*claims.IssuedAt, 0)
	expiresAt := time.Unix(*claims.ExpiresAt, 0)
	if issuedAt.After(now.Add(clockLeeway)) || !expiresAt.After(issuedAt) ||
		expiresAt.Sub(issuedAt) > maximumLifetime || !now.Before(expiresAt.Add(clockLeeway)) {
		return time.Time{}, false
	}
	return expiresAt.Add(clockLeeway), validContext(*claims.Context)
}

// audienceIncludesGateway accepts the audience as a string or an array of strings (RFC 7519).
func audienceIncludesGateway(raw json.RawMessage) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == expectedAudience
	}
	var several []string
	if json.Unmarshal(raw, &several) == nil {
		for _, audience := range several {
			if audience == expectedAudience {
				return true
			}
		}
	}
	return false
}

func validContext(operator contracts.OperatorContext) bool {
	if !uuidPattern.MatchString(operator.UserID) || !uuidPattern.MatchString(operator.OrganizationID) ||
		operator.Roles == nil || len(operator.Roles) > maximumRoles {
		return false
	}
	seenRoles := make(map[string]bool, len(operator.Roles))
	for _, role := range operator.Roles {
		if role == "" || len(role) > maximumRoleLength || !utf8.ValidString(role) || seenRoles[role] ||
			strings.ContainsFunc(role, func(character rune) bool { return character < 0x20 || character == 0x7f }) {
			return false
		}
		seenRoles[role] = true
	}
	return true
}

// rememberTokenID records a token id until its expiry and refuses one already used.
func (verifier *Verifier) rememberTokenID(tokenID string, forgetAt, now time.Time) bool {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	if rememberedUntil, used := verifier.usedTokenIDs[tokenID]; used && now.Before(rememberedUntil) {
		return false
	}
	if len(verifier.usedTokenIDs) >= maximumRememberedTokens {
		for rememberedID, rememberedUntil := range verifier.usedTokenIDs {
			if !now.Before(rememberedUntil) {
				delete(verifier.usedTokenIDs, rememberedID)
			}
		}
		// Still full: refuse rather than forget a token that could be replayed.
		if len(verifier.usedTokenIDs) >= maximumRememberedTokens {
			return false
		}
	}
	verifier.usedTokenIDs[tokenID] = forgetAt
	return true
}

// decodeSegment decodes one base64url JWT segment strictly: no unknown fields, no trailing data.
func decodeSegment(segment string, target any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil || !utf8.Valid(decoded) {
		return ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.More() {
		return ErrInvalid
	}
	return nil
}

type contextKey struct{}

// WithOperator returns a context carrying the verified operator.
func WithOperator(ctx context.Context, operator contracts.OperatorContext) context.Context {
	return context.WithValue(ctx, contextKey{}, operator)
}

// FromContext returns the verified operator of a request; false means none was verified, and
// the caller must refuse the command.
func FromContext(ctx context.Context) (contracts.OperatorContext, bool) {
	operator, ok := ctx.Value(contextKey{}).(contracts.OperatorContext)
	return operator, ok
}
