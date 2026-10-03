package security

import (
	"os"
	"testing"
)

// TestSuiteInjectedFailure lets `pnpm verify:controls` show that one failing case makes the whole
// suite exit nonzero: with VERIFY_CONTROLS_INJECT_FAILURE=1 it fails through the real go test path.
// Unset, it passes (never skips, so the database test command keeps reporting a clean pass).
func TestSuiteInjectedFailure(t *testing.T) {
	if os.Getenv("VERIFY_CONTROLS_INJECT_FAILURE") == "1" {
		t.Fatal("deliberately injected failure (VERIFY_CONTROLS_INJECT_FAILURE=1)")
	}
}
