package security

import (
	"testing"

	"starter/services/gateway/internal/contracts"
)

// GO-58: every reason the security controls hand to a decision or an event is in the X-13
// vocabulary and has its fixed safe message, so no security stop reaches an operator unreadable.
func TestEverySecurityDecisionReasonIsAnX13CodeWithASafeMessage(t *testing.T) {
	reasons := []string{
		ReasonContentRedacted, ReasonContentBlocked, ReasonContentTooLarge, ReasonSignatureMatch,
		ReasonSemanticInjectionDetected, ReasonSecurityEvaluatorUnavailable, ReasonSecurityAllowanceExhausted,
	}
	for _, reason := range reasons {
		code := contracts.ReasonCode(reason)
		if !code.Valid() || code.SafeMessage() == "" {
			t.Errorf("security reason %q is not an X-13 code with a safe message", reason)
		}
	}
	// The one other reason string is evidence on a control record, never a decision reason.
	if contracts.ReasonCode(ReasonNoFreeTextArguments).Valid() {
		t.Errorf("%q is a control-record reason and must not be a decision reason", ReasonNoFreeTextArguments)
	}
}
