package agent

import "testing"

// mustScan reads one row and fails the test when the read errors, so a failed query can never pass
// as a zero count or an empty value.
func mustScan(t *testing.T, row interface{ Scan(...any) error }, destinations ...any) {
	t.Helper()
	if err := row.Scan(destinations...); err != nil {
		t.Fatalf("read: %v", err)
	}
}
