package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestSecretNeverPrintsItsValue(t *testing.T) {
	const sensitiveValue = "sensitive-value-0123456789"
	secret := NewSecret(sensitiveValue)

	var logOutput bytes.Buffer
	logger := New(&logOutput, slog.LevelDebug)
	logger.Info("settings", "secret", secret, "nested", struct{ Token Secret }{secret})

	encodedJSON, err := json.Marshal(map[string]any{"secret": secret})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	renderings := map[string]string{
		"slog":          logOutput.String(),
		"fmt %s":        fmt.Sprintf("%s", secret),
		"fmt %v":        fmt.Sprintf("%v", secret),
		"fmt %+v":       fmt.Sprintf("%+v", struct{ Token Secret }{secret}),
		"fmt %#v":       fmt.Sprintf("%#v", secret),
		"encoding/json": string(encodedJSON),
	}
	for rendererName, rendered := range renderings {
		if strings.Contains(rendered, sensitiveValue) {
			t.Errorf("%s leaks the secret: %s", rendererName, rendered)
		}
	}
	if secret.Reveal() != sensitiveValue {
		t.Error("Reveal must return the original value")
	}
}

func TestParseLevelRejectsUnknownNames(t *testing.T) {
	if _, err := ParseLevel("verbose"); err == nil {
		t.Error("expected an error for an unknown level")
	}
	level, err := ParseLevel(" WARN ")
	if err != nil || level != slog.LevelWarn {
		t.Errorf("ParseLevel(WARN) = %v, %v", level, err)
	}
}
