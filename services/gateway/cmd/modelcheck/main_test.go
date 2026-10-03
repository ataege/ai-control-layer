package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"starter/services/gateway/internal/config"
)

func diagnosticLookup(baseURL string) config.LookupFunc {
	return func(name string) (string, bool) {
		switch name {
		case "MODEL_BASE_URL":
			return baseURL, true
		case "MODEL_NAME":
			return "synthetic-fixture", true
		default:
			return "", false
		}
	}
}

// This provider double verifies diagnostic plumbing, not model capability.
func TestDiagnosticCallsBothPurposes(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Model   string         `json:"model"`
			Stream  bool           `json:"stream"`
			Think   *bool          `json:"think"`
			Format  map[string]any `json:"format"`
			Tools   []any          `json:"tools"`
			Options struct {
				Context int `json:"num_ctx"`
				Output  int `json:"num_predict"`
			} `json:"options"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || r.Method != "POST" || r.URL.Path != "/api/chat" || request.Model != "synthetic-fixture" || request.Stream || request.Think == nil || *request.Think || request.Format["type"] != "object" || len(request.Tools) != 0 || request.Options.Context != 4096 || request.Options.Output != 256 {
			t.Error("invalid diagnostic provider request")
		}
		_, _ = w.Write([]byte(`{"model":"synthetic-fixture","done":true,"message":{"role":"assistant","content":"{\"status\":\"ok\"}"},"prompt_eval_count":1,"eval_count":2}`))
	}))
	defer server.Close()
	var output bytes.Buffer
	if err := run(context.Background(), diagnosticLookup(server.URL), &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "provider_duration_ns") {
		t.Fatal("missing provider duration was presented as a measurement")
	}
	if calls.Load() != 2 || strings.Count(output.String(), `"status":"PASS"`) != 2 || !strings.Contains(output.String(), `"purpose":"agent"`) || !strings.Contains(output.String(), `"purpose":"security"`) {
		t.Fatalf("calls=%d output=%s", calls.Load(), output.String())
	}
}

func TestDiagnosticFailsWithoutLeakingOrRetrying(t *testing.T) {
	for _, tc := range []struct{ name, content, usage string }{
		{"unknown field", `{"status":"ok","sensitive":"provider-secret"}`, `,"prompt_eval_count":1,"eval_count":2`},
		{"duplicate field", `{"status":"bad","status":"ok"}`, `,"prompt_eval_count":1,"eval_count":2`},
		{"missing usage", `{"status":"ok"}`, ""},
		{"trailing content", `{"status":"ok"} provider-secret`, `,"prompt_eval_count":1,"eval_count":2`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				encoded, _ := json.Marshal(tc.content)
				_, _ = w.Write([]byte(`{"model":"synthetic-fixture","done":true,"message":{"role":"assistant","content":` + string(encoded) + `}` + tc.usage + `}`))
			}))
			defer server.Close()
			var output bytes.Buffer
			if err := run(context.Background(), diagnosticLookup(server.URL), &output); err == nil {
				t.Fatal("invalid diagnostic passed")
			}
			if calls.Load() != 1 || strings.Contains(output.String(), "PASS") || strings.Contains(output.String(), "provider-secret") {
				t.Fatalf("calls=%d output=%s", calls.Load(), output.String())
			}
		})
	}
}

func TestDiagnosticMissingConfigDoesNotDispatch(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), func(string) (string, bool) { return "", false }, &output); err == nil {
		t.Fatal("missing configuration accepted")
	}
	if strings.Contains(output.String(), "PASS") {
		t.Fatalf("false pass: %s", output.String())
	}
}
