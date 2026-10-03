package model

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// These are transport fixtures, not evidence of a real model's detection quality.
const modelFixture = `{"model":"fixture-model:test","done":true,"message":{"role":"assistant","content":"fixture"},"prompt_eval_count":12,"eval_count":3,"total_duration":1000}`

func fixtureClient(t *testing.T, handler http.HandlerFunc, mutate func(*Options)) *Ollama {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	options := Options{BaseURL: server.URL, Model: "fixture-model:test", Timeout: time.Second, MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20}
	if mutate != nil {
		mutate(&options)
	}
	client, err := NewOllama(options)
	if err != nil {
		t.Fatalf("NewOllama: %v", err)
	}
	return client
}

func fixtureRequest() Request {
	return Request{Purpose: AgentPurpose, Messages: []Message{{Role: "user", Content: "synthetic fixture"}}, ContextTokens: 1024, OutputTokens: 64}
}

func TestOllamaRequestAndUsage(t *testing.T) {
	for _, purpose := range []Purpose{AgentPurpose, SecurityPurpose} {
		t.Run(string(purpose), func(t *testing.T) {
			var calls atomic.Int32
			client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/api/chat" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["model"] != "fixture-model:test" || body["stream"] != false {
					t.Errorf("model/stream: %v", body)
				}
				options, ok := body["options"].(map[string]any)
				if !ok || options["num_ctx"] != float64(1024) || options["num_predict"] != float64(64) {
					t.Errorf("token bounds: %v", body["options"])
				}
				if _, ok := body["format"].(map[string]any); !ok {
					t.Errorf("schema was not forwarded: %v", body["format"])
				}
				_, _ = w.Write([]byte(modelFixture))
			}, nil)
			request := fixtureRequest()
			request.Purpose = purpose
			request.Format = json.RawMessage(`{"type":"object","properties":{"safe":{"type":"boolean"}}}`)
			result, err := client.Chat(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if result.Message.Content != "fixture" || result.Usage.InputTokens == nil || *result.Usage.InputTokens != 12 || result.Usage.OutputTokens == nil || *result.Usage.OutputTokens != 3 {
				t.Fatalf("unexpected result: %+v", result)
			}
			if calls.Load() != 1 {
				t.Fatalf("calls = %d", calls.Load())
			}
		})
	}
}

func TestOllamaMissingUsageIsNotZero(t *testing.T) {
	for _, tc := range []struct {
		name, counts string
		present      bool
	}{
		{"missing", "", false}, {"explicit zero", `,"prompt_eval_count":0,"eval_count":0`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"model":"fixture-model:test","done":true,"message":{"role":"assistant","content":"fixture"}` + tc.counts + `}`))
			}, nil)
			result, err := client.Chat(context.Background(), fixtureRequest())
			if err != nil {
				t.Fatal(err)
			}
			if (result.Usage.InputTokens != nil) != tc.present || (result.Usage.OutputTokens != nil) != tc.present {
				t.Fatalf("usage: %+v", result.Usage)
			}
			if tc.present && (*result.Usage.InputTokens != 0 || *result.Usage.OutputTokens != 0) {
				t.Fatalf("usage: %+v", result.Usage)
			}
		})
	}
}

func TestOllamaToolCallsRemainUntrustedProposals(t *testing.T) {
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Tools []Tool `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body.Tools) != 1 || body.Tools[0].Function.Name != "read_invoice" {
			t.Errorf("tool definitions: %+v", body.Tools)
		}
		_, _ = w.Write([]byte(`{"model":"fixture-model:test","done":true,"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"unregistered_tool","arguments":{"invoice_id":"synthetic-1"}}},{"function":{"name":"queue_report","arguments":{"report_id":"synthetic-2"}}}]}}`))
	}, nil)
	request := fixtureRequest()
	request.Tools = []Tool{{Type: "function", Function: FunctionDefinition{Name: "read_invoice", Parameters: json.RawMessage(`{"type":"object"}`)}}}
	result, err := client.Chat(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Message.ToolCalls) != 2 || result.Message.ToolCalls[0].Function.Name != "unregistered_tool" || string(result.Message.ToolCalls[1].Function.Arguments) != `{"report_id":"synthetic-2"}` {
		t.Fatalf("proposals lost: %+v", result.Message)
	}
}

func TestOllamaRejectsInvalidConfiguration(t *testing.T) {
	for _, base := range []string{"", "file:///tmp/model", "http://user:password@localhost:11434", "http://localhost:11434/private", "http://localhost:11434?token=secret", "http://localhost:11434#secret"} {
		if _, err := NewOllama(Options{BaseURL: base, Model: "fixture-model:test", Timeout: time.Second, MaxRequestBytes: 1024, MaxResponseBytes: 1024}); !errors.Is(err, ErrConfiguration) {
			t.Errorf("URL %q: %v", base, err)
		}
	}
}

func TestOllamaRejectsUnsafeResponsesWithoutRetry(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"case duplicate done", strings.Replace(modelFixture, `"done":true`, `"done":false,"Done":true`, 1), 200},
		{"case duplicate usage", strings.Replace(modelFixture, `"eval_count":3`, `"eval_count":-1,"EVAL_COUNT":3`, 1), 200},
		{"duplicate done", strings.Replace(modelFixture, `"done":true`, `"done":false,"done":true`, 1), 200},
		{"null usage", strings.Replace(modelFixture, `"eval_count":3`, `"eval_count":null`, 1), 200},
		{"fraction usage", strings.Replace(modelFixture, `"eval_count":3`, `"eval_count":1.5`, 1), 200},
		{"string usage", strings.Replace(modelFixture, `"eval_count":3`, `"eval_count":"3"`, 1), 200},
		{"overflow usage", strings.Replace(modelFixture, `"eval_count":3`, `"eval_count":9223372036854775808`, 1), 200},
		{"unknown role", strings.Replace(modelFixture, `"role":"assistant"`, `"role":"admin"`, 1), 200},
		{"malformed", `{`, 200}, {"trailing document", modelFixture + `{}`, 200},
		{"wrong model", strings.Replace(modelFixture, "fixture-model:test", "other-model", 1), 200},
		{"incomplete", strings.Replace(modelFixture, `"done":true`, `"done":false`, 1), 200},
		{"negative usage", strings.Replace(modelFixture, `"eval_count":3`, `"eval_count":-1`, 1), 200},
		{"server failure", `sensitive-provider-body`, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			client := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}, nil)
			_, err := client.Chat(context.Background(), fixtureRequest())
			if err == nil {
				t.Fatal("unsafe response accepted")
			}
			if strings.Contains(err.Error(), "sensitive-provider-body") {
				t.Fatal("provider body leaked")
			}
			if calls.Load() != 1 {
				t.Fatalf("retry: calls = %d", calls.Load())
			}
		})
	}
}

func TestOllamaBoundsAndInvalidRequestDoNotDispatch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Request)
		limit  int64
	}{
		{"unknown purpose", func(r *Request) { r.Purpose = "unknown" }, 1 << 20},
		{"unbounded output", func(r *Request) { r.OutputTokens = 0 }, 1 << 20},
		{"unbounded context", func(r *Request) { r.ContextTokens = 0 }, 1 << 20},
		{"invalid schema", func(r *Request) { r.Format = json.RawMessage(`{`) }, 1 << 20},
		{"oversized request", func(r *Request) { r.Messages[0].Content = strings.Repeat("x", 1024) }, 64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			client := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); _, _ = w.Write([]byte(modelFixture)) }, func(o *Options) { o.MaxRequestBytes = tc.limit })
			request := fixtureRequest()
			tc.mutate(&request)
			_, err := client.Chat(context.Background(), request)
			if !errors.Is(err, ErrRequest) {
				t.Fatalf("expected ErrRequest, got %v", err)
			}
			if calls.Load() != 0 {
				t.Fatal("invalid request dispatched")
			}
		})
	}
	client := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(modelFixture)) }, func(o *Options) { o.MaxResponseBytes = 32 })
	if _, err := client.Chat(context.Background(), fixtureRequest()); !errors.Is(err, ErrResponse) {
		t.Fatalf("oversized response: %v", err)
	}
}

func TestOllamaDoesNotFollowRedirect(t *testing.T) {
	var leaked atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { leaked.Add(1); _, _ = w.Write([]byte(modelFixture)) }))
	defer target.Close()
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}, nil)
	if _, err := client.Chat(context.Background(), fixtureRequest()); err == nil {
		t.Fatal("redirect accepted")
	}
	if leaked.Load() != 0 {
		t.Fatal("model payload sent to redirect destination")
	}
}

func TestOllamaTimeoutAndCancellation(t *testing.T) {
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
	}, func(o *Options) { o.Timeout = 20 * time.Millisecond })
	if _, err := client.Chat(context.Background(), fixtureRequest()); !errors.Is(err, ErrTimeout) {
		t.Fatalf("timeout: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Chat(ctx, fixtureRequest()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request: %v", err)
	}
}
