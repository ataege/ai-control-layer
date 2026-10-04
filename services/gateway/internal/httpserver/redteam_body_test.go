package httpserver

import (
	"net/http/httptest"
	"strings"
	"testing"
)

type bodyTarget struct {
	Decision string `json:"decision"`
}

// TestBodyDecoderRefusesEverythingAfterTheDocument (lane w2): one document, then the end of the
// input. A stray closing brace or bracket after the document used to pass `decoder.More`.
func TestBodyDecoderRefusesEverythingAfterTheDocument(t *testing.T) {
	for name, body := range map[string]string{
		"closing brace":     `{"decision":"approve"}}`,
		"closing bracket":   `{"decision":"approve"}]`,
		"several braces":    `{"decision":"approve"}}}}`,
		"second document":   `{"decision":"approve"}{"decision":"reject"}`,
		"garbage":           `{"decision":"approve"} x`,
		"comma":             `{"decision":"approve"},`,
		"nul":               "{\"decision\":\"approve\"}\x00",
		"second line":       "{\"decision\":\"approve\"}\n{}",
		"byte order mark":   "\ufeff{\"decision\":\"approve\"}",
		"leading garbage":   `x{"decision":"approve"}`,
		"array":             `[{"decision":"approve"}]`,
		"empty":             ``,
		"truncated":         `{"decision":"approve"`,
		"unknown field":     `{"decision":"approve","grant":true}`,
		"nesting bomb":      `{"decision":` + strings.Repeat("[", 100000) + strings.Repeat("]", 100000) + `}`,
		"number for string": `{"decision":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			var target bodyTarget
			err := decodeJSONBody(httptest.NewRecorder(), httptest.NewRequest("POST", "/x", strings.NewReader(body)), 1<<20, &target)
			if err == nil {
				t.Fatalf("accepted %q as %+v", body, target)
			}
		})
	}
	t.Run("document with surrounding white space is accepted", func(t *testing.T) {
		var target bodyTarget
		body := " \n{\"decision\":\"approve\"}\r\n\t "
		if err := decodeJSONBody(httptest.NewRecorder(), httptest.NewRequest("POST", "/x", strings.NewReader(body)), 1<<20, &target); err != nil || target.Decision != "approve" {
			t.Fatalf("%v %+v", err, target)
		}
	})
}
