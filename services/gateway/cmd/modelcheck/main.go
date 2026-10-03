// modelcheck makes explicit diagnostic provider calls. It is not a governed task
// and does not prove budgeting, agent execution or semantic detection quality.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/model"
)

var probeSchema = json.RawMessage(`{"type":"object","properties":{"status":{"type":"string","enum":["ok"]}},"required":["status"],"additionalProperties":false}`)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if run(ctx, os.LookupEnv, os.Stdout) != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, lookup config.LookupFunc, output io.Writer) error {
	settings, err := config.LoadModelFrom(lookup)
	if err != nil {
		_, _ = fmt.Fprintln(output, "diagnostic provider call: FAIL (model configuration)")
		return err
	}
	client, err := model.NewOllama(model.Options{BaseURL: settings.BaseURL, Model: settings.Name, Timeout: 30 * time.Second, MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		_, _ = fmt.Fprintln(output, "diagnostic provider call: FAIL (model configuration)")
		return err
	}
	thinking := false
	for _, purpose := range []model.Purpose{model.AgentPurpose, model.SecurityPurpose} {
		limit := model.DefaultAccountingSettings().AgentOutputTokens
		if purpose == model.SecurityPurpose {
			limit = model.DefaultAccountingSettings().SecurityOutputTokens
		}
		result, err := client.Chat(ctx, model.Request{Purpose: purpose, Think: &thinking, ContextTokens: 4096, OutputTokens: limit, Format: probeSchema, Messages: []model.Message{{Role: "user", Content: `Synthetic connectivity diagnostic. Return only the JSON object {"status":"ok"}. This is not a task or a security assessment.`}}})
		if err != nil || len(result.Message.ToolCalls) != 0 || !validProbe(result.Message.Content) || result.Usage.InputTokens == nil || result.Usage.OutputTokens == nil {
			_, _ = fmt.Fprintf(output, "diagnostic provider call (%s): FAIL (invalid or unavailable provider response)\n", purpose)
			return fmt.Errorf("diagnostic provider call failed")
		}
		record := struct {
			Label              string        `json:"label"`
			Purpose            model.Purpose `json:"purpose"`
			Status             string        `json:"status"`
			Usage              model.Usage   `json:"usage"`
			DurationNS         int64         `json:"duration_ns"`
			ProviderDurationNS *int64        `json:"provider_duration_ns,omitempty"`
		}{Label: "diagnostic provider call", Purpose: purpose, Status: "PASS", Usage: result.Usage, DurationNS: int64(result.Duration)}
		if result.ProviderDurationKnown {
			providerDuration := int64(result.ProviderDuration)
			record.ProviderDurationNS = &providerDuration
		}
		if err := json.NewEncoder(output).Encode(record); err != nil {
			return err
		}
	}
	return nil
}

func validProbe(content string) bool {
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return false
	}
	if !decoder.More() {
		return false
	}
	key, err := decoder.Token()
	if err != nil || key != "status" {
		return false
	}
	var status string
	if decoder.Decode(&status) != nil || status != "ok" || decoder.More() {
		return false
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return false
	}
	var trailing any
	return decoder.Decode(&trailing) == io.EOF
}
