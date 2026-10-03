// Package model provides model transport only. Callers remain responsible for
// authorization, reservations, purpose budgets and interpreting tool proposals.
package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Purpose labels a locally metered use; it is not sent as remote metadata.
type Purpose string

const (
	AgentPurpose    Purpose = "agent"
	SecurityPurpose Purpose = "security"
)

var (
	ErrConfiguration = errors.New("invalid model configuration")
	ErrRequest       = errors.New("invalid model request")
	ErrTransport     = errors.New("model transport failed")
	ErrResponse      = errors.New("invalid model response")
	ErrTimeout       = errors.New("model request timed out")
)

type Options struct {
	BaseURL, Model                    string
	Timeout                           time.Duration
	MaxRequestBytes, MaxResponseBytes int64
}

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ToolCall struct {
	Function FunctionCall `json:"function"`
}
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type Request struct {
	Purpose       Purpose         `json:"purpose"`
	Messages      []Message       `json:"messages"`
	ContextTokens int             `json:"context_tokens"`
	OutputTokens  int             `json:"output_tokens"`
	Format        json.RawMessage `json:"format,omitempty"`
	Tools         []Tool          `json:"tools,omitempty"`
}

// Usage keeps absent provider counts unknown through nil pointers.
type Usage struct {
	InputTokens  *int64 `json:"input_tokens"`
	OutputTokens *int64 `json:"output_tokens"`
}

// Result can retain known usage even when Chat returns a response error.
type Result struct {
	Message          Message       `json:"message"`
	Usage            Usage         `json:"usage"`
	Duration         time.Duration `json:"duration"`
	ProviderDuration time.Duration `json:"provider_duration"`
}

type Ollama struct {
	endpoint                    string
	model                       string
	timeout                     time.Duration
	requestLimit, responseLimit int64
	client                      *http.Client
}

func NewOllama(options Options) (*Ollama, error) {
	parsedURL, err := url.Parse(options.BaseURL)
	if err != nil || parsedURL == nil {
		return nil, ErrConfiguration
	}
	if (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") ||
		parsedURL.Hostname() == "" || parsedURL.User != nil ||
		parsedURL.RawQuery != "" || parsedURL.ForceQuery ||
		parsedURL.Fragment != "" || parsedURL.RawFragment != "" ||
		(parsedURL.Path != "" && parsedURL.Path != "/") || parsedURL.RawPath != "" {
		return nil, ErrConfiguration
	}
	if options.Model == "" || strings.TrimSpace(options.Model) != options.Model || options.Timeout <= 0 {
		return nil, ErrConfiguration
	}
	// Limits keep allocations and the limit+1 overflow bounded explicitly.
	if options.MaxRequestBytes <= 0 || options.MaxResponseBytes <= 0 ||
		options.MaxRequestBytes > 64<<20 || options.MaxResponseBytes > 64<<20 {
		return nil, ErrConfiguration
	}
	parsedURL.Path = "/api/chat"
	return &Ollama{
		endpoint:      parsedURL.String(),
		model:         options.Model,
		timeout:       options.Timeout,
		requestLimit:  options.MaxRequestBytes,
		responseLimit: options.MaxResponseBytes,
		client: &http.Client{
			Transport:     &http.Transport{Proxy: nil},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func validObject(raw json.RawMessage) bool {
	return uniqueJSON(raw) && len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '{'
}

func validMessage(message Message) bool {
	switch message.Role {
	case "system", "user", "assistant", "tool":
	default:
		return false
	}
	if len(message.ToolCalls) > 0 && message.Role != "assistant" {
		return false
	}
	for _, call := range message.ToolCalls {
		if strings.TrimSpace(call.Function.Name) == "" || !validObject(call.Function.Arguments) {
			return false
		}
	}
	return message.Content != "" || len(message.ToolCalls) > 0
}

func (client *Ollama) Chat(ctx context.Context, request Request) (Result, error) {
	var result Result
	if ctx == nil || (request.Purpose != AgentPurpose && request.Purpose != SecurityPurpose) || len(request.Messages) == 0 || request.ContextTokens <= 0 || request.OutputTokens <= 0 || request.OutputTokens > request.ContextTokens {
		return result, ErrRequest
	}
	for _, message := range request.Messages {
		if !validMessage(message) {
			return result, ErrRequest
		}
	}
	if len(request.Format) > 0 && !validObject(request.Format) && string(request.Format) != `"json"` {
		return result, ErrRequest
	}
	for _, tool := range request.Tools {
		if tool.Type != "function" || strings.TrimSpace(tool.Function.Name) == "" || !validObject(tool.Function.Parameters) {
			return result, ErrRequest
		}
	}
	payload := struct {
		Model    string          `json:"model"`
		Messages []Message       `json:"messages"`
		Stream   bool            `json:"stream"`
		Format   json.RawMessage `json:"format,omitempty"`
		Tools    []Tool          `json:"tools,omitempty"`
		Options  struct {
			Context int `json:"num_ctx"`
			Output  int `json:"num_predict"`
		} `json:"options"`
	}{Model: client.model, Messages: request.Messages, Format: request.Format, Tools: request.Tools}
	payload.Options.Context, payload.Options.Output = request.ContextTokens, request.OutputTokens
	body, err := json.Marshal(payload)
	if err != nil || int64(len(body)) > client.requestLimit {
		return result, ErrRequest
	}
	requestContext, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestContext, http.MethodPost, client.endpoint, bytes.NewReader(body))
	if err != nil {
		return result, ErrRequest
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	started := time.Now()
	response, err := client.client.Do(httpRequest)
	result.Duration = time.Since(started)
	if err != nil {
		return result, transportError(ctx, requestContext)
	}
	defer response.Body.Close()
	body, err = io.ReadAll(io.LimitReader(response.Body, client.responseLimit+1))
	result.Duration = time.Since(started)
	if err != nil {
		return result, transportError(ctx, requestContext)
	}
	if int64(len(body)) > client.responseLimit {
		return result, ErrResponse
	}
	var envelope struct {
		Model   string          `json:"model"`
		Message Message         `json:"message"`
		Done    bool            `json:"done"`
		Error   json.RawMessage `json:"error"`
	}
	if !validObject(body) {
		return result, ErrResponse
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(body, &fields)
	var usageError bool
	result.Usage.InputTokens, err = count(fields["prompt_eval_count"])
	usageError = err != nil
	result.Usage.OutputTokens, err = count(fields["eval_count"])
	usageError = usageError || err != nil
	providerDuration, err := count(fields["total_duration"])
	if providerDuration != nil {
		result.ProviderDuration = time.Duration(*providerDuration)
	}
	if !exactResponseKeys(body) || json.Unmarshal(body, &envelope) != nil || response.StatusCode != http.StatusOK || usageError || err != nil || len(envelope.Error) > 0 || !envelope.Done || envelope.Model != client.model || envelope.Message.Role != "assistant" || !validMessage(envelope.Message) {
		return result, ErrResponse
	}
	result.Message = envelope.Message
	return result, nil
}

func count(raw json.RawMessage) (*int64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value int64
	if string(raw) == "null" || json.Unmarshal(raw, &value) != nil || value < 0 {
		return nil, ErrResponse
	}
	return &value, nil
}

func transportError(parent, request context.Context) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	if errors.Is(request.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	return ErrTransport
}

// Reject ambiguous duplicate keys at every nesting level before typed decoding.
func uniqueJSON(raw []byte) bool {
	if !json.Valid(raw) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value func() error
	value = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		switch delimiter {
		case '{':
			keys := make(map[string]bool)
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || keys[name] {
					return ErrResponse
				}
				keys[name] = true
				if err := value(); err != nil {
					return err
				}
			}
			token, err = decoder.Token()
			if err != nil || token != json.Delim('}') {
				return ErrResponse
			}
		case '[':
			for decoder.More() {
				if err := value(); err != nil {
					return err
				}
			}
			token, err = decoder.Token()
			if err != nil || token != json.Delim(']') {
				return ErrResponse
			}
		default:
			return ErrResponse
		}
		return nil
	}
	if value() != nil {
		return false
	}
	_, err := decoder.Token()
	return errors.Is(err, io.EOF)
}

// encoding/json matches field names without regard to case. The wire contract
// must not let aliases overwrite validated metadata or tool proposals.
func exactKeys(raw json.RawMessage, known ...string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return false
	}
	for name := range fields {
		for _, expected := range known {
			if strings.EqualFold(name, expected) && name != expected {
				return false
			}
		}
	}
	return true
}

func exactResponseKeys(raw json.RawMessage) bool {
	if !exactKeys(raw, "model", "message", "done", "error", "prompt_eval_count", "eval_count", "total_duration") {
		return false
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	message := fields["message"]
	if !exactKeys(message, "role", "content", "tool_calls") {
		return false
	}
	var messageFields map[string]json.RawMessage
	_ = json.Unmarshal(message, &messageFields)
	if callsRaw, exists := messageFields["tool_calls"]; exists {
		var calls []json.RawMessage
		if json.Unmarshal(callsRaw, &calls) != nil {
			return false
		}
		for _, call := range calls {
			if !exactKeys(call, "function") {
				return false
			}
			var callFields map[string]json.RawMessage
			_ = json.Unmarshal(call, &callFields)
			if !exactKeys(callFields["function"], "name", "arguments") {
				return false
			}
		}
	}
	return true
}
