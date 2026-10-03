package config

import (
	"errors"
	"strings"
	"testing"
)

func modelLookup(values map[string]string) LookupFunc {
	return func(name string) (string, bool) { value, found := values[name]; return value, found }
}

func TestLoadModelExplicitSettings(t *testing.T) {
	configuration, err := LoadModelFrom(modelLookup(map[string]string{
		"MODEL_BASE_URL": "http://192.168.1.20:11434/",
		"MODEL_NAME":     "custom-model:changed",
	}))
	if err != nil || configuration.BaseURL != "http://192.168.1.20:11434/" || configuration.Name != "custom-model:changed" {
		t.Fatalf("explicit settings not retained: %+v, %v", configuration, err)
	}
	for _, baseURL := range []string{"", "  "} {
		configuration, err = LoadModelFrom(modelLookup(map[string]string{"MODEL_NAME": "another:tag", "MODEL_BASE_URL": baseURL}))
		if err != nil || configuration.BaseURL != "http://127.0.0.1:11434" {
			t.Fatalf("default origin missing: %+v, %v", configuration, err)
		}
	}
}

func TestLoadModelRejectsMissingOrAmbiguousTag(t *testing.T) {
	for _, name := range []string{"", " ", " qwen:3b", "qwen:3b ", "qwen\n3b", "qwen\x003b", "qwen\t3b"} {
		_, err := LoadModelFrom(modelLookup(map[string]string{"MODEL_NAME": name}))
		var validationError *ValidationError
		if !errors.As(err, &validationError) || !strings.Contains(err.Error(), "MODEL_NAME") {
			t.Fatalf("invalid model tag accepted")
		}
	}
}

func TestLoadModelRejectsUnsafeOriginsWithoutDisclosure(t *testing.T) {
	for _, baseURL := range []string{
		"file:///private/value", "http://user:private-secret@localhost:11434",
		"http://localhost:11434/api/chat", "http://localhost:11434?secret=private-secret",
		"http://localhost:11434#private-secret", "http://localhost:11434#", "http://localhost:11434?", "http://localhost:0", "http://localhost:65536",
		"http://localhost:", "http://localhost:invalid", "http:///", " http://localhost:11434",
	} {
		_, err := LoadModelFrom(modelLookup(map[string]string{"MODEL_NAME": "fixture:tag", "MODEL_BASE_URL": baseURL}))
		var validationError *ValidationError
		if !errors.As(err, &validationError) || !strings.Contains(err.Error(), "MODEL_BASE_URL") {
			t.Fatalf("unsafe model origin accepted")
		}
		if strings.Contains(err.Error(), "private-secret") || strings.Contains(err.Error(), baseURL) {
			t.Fatal("configuration error disclosed its supplied value")
		}
	}
}
