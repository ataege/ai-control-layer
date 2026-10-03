package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// A missing or extra argument reaches neither the configuration nor the database.
func TestUsageErrorsRunNothing(t *testing.T) {
	for _, arguments := range [][]string{
		nil,
		{"-run", "4c1f5a0e-0000-4000-8000-000000000001"},
		{"-fixture", "hostile_note_redirect_record_v1"},
		{"-run", "4c1f5a0e-0000-4000-8000-000000000001", "-fixture", "hostile_note_redirect_record_v1", "extra"},
		{"-unknown"},
	} {
		var output, errorOutput bytes.Buffer
		if code := run(context.Background(), arguments, &output, &errorOutput); code != exitNotRun || output.Len() != 0 ||
			!strings.Contains(errorOutput.String(), "usage: replay") {
			t.Fatalf("%v: exit %d, output %q, errors %q", arguments, code, output.String(), errorOutput.String())
		}
	}
}
