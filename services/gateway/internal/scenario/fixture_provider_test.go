// Package scenario holds the core-story proofs (GO-66, GO-67, GO-47, GO-56): the reconciliation
// task driven through the production chain (agent.NewProductionChain), from real admission to the
// review wait and, once the resume lands (GO-40), to the queued vendor report.
package scenario

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"starter/services/gateway/internal/contracts"
)

// fixtureVerdict is the scripted security answer: a valid, low-risk verdict.
const fixtureVerdict = `{"risk_category":"none","score":0.02,"reason_code":"no_risk_found"}`

// fixtureProvider is a scripted, labelled stand-in for the Ollama server. It is not a model: it
// proposes the story's next step from the request it receives and answers every security request
// with fixtureVerdict. It records every request body so the test can inspect what the model would
// have seen.
type fixtureProvider struct {
	t          *testing.T
	modelName  string
	invoiceIDs []string
	vendorID   string
	recipient  string

	mutex            sync.Mutex
	agentRequests    [][]byte
	securityRequests [][]byte
}

type chatRequest struct {
	Model    string          `json:"model"`
	Messages []chatMessage   `json:"messages"`
	Format   json.RawMessage `json:"format"`
	Tools    json.RawMessage `json:"tools"`
}

type chatMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolCalls []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	} `json:"tool_calls"`
}

func (provider *fixtureProvider) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var body []byte
	body, request.Body = readBody(request)
	var chat chatRequest
	if json.Unmarshal(body, &chat) != nil || chat.Model != provider.modelName {
		http.Error(writer, "fixture: unexpected request", http.StatusBadRequest)
		return
	}
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	if len(chat.Format) > 0 && string(chat.Format) != "null" {
		provider.securityRequests = append(provider.securityRequests, body)
		provider.answer(writer, map[string]any{"role": "assistant", "content": fixtureVerdict})
		return
	}
	provider.agentRequests = append(provider.agentRequests, body)
	message, ok := provider.nextStep(chat.Messages)
	if !ok {
		http.Error(writer, "fixture: the script has no further step", http.StatusBadRequest)
		return
	}
	provider.answer(writer, message)
}

// nextStep picks the story step from the number of calls already in the context (a denied call
// is stored too), and takes report ids from the create_report results the context holds.
func (provider *fixtureProvider) nextStep(messages []chatMessage) (map[string]any, bool) {
	step := 0
	reportByTemplate := map[string]string{}
	for _, message := range messages {
		if message.Role == "assistant" && len(message.ToolCalls) > 0 {
			step++
		}
		if message.Role == "tool" {
			var result struct {
				ReportID string `json:"report_id"`
				Template string `json:"template"`
			}
			if json.Unmarshal([]byte(message.Content), &result) == nil && result.ReportID != "" {
				reportByTemplate[result.Template] = result.ReportID
			}
		}
	}
	call := func(tool string, arguments map[string]any) map[string]any {
		return map[string]any{"role": "assistant", "content": "",
			"tool_calls": []any{map[string]any{"function": map[string]any{"name": tool, "arguments": arguments}}}}
	}
	sources := provider.invoiceIDs
	switch step {
	case 0:
		return call("read_invoice", map[string]any{"invoice_id": sources[0]}), true
	case 1:
		return call("read_invoice", map[string]any{"invoice_id": sources[1]}), true
	case 2:
		return call("read_vendor", map[string]any{"vendor_id": provider.vendorID}), true
	case 3:
		return call("create_report", map[string]any{"template": string(contracts.TemplateInternalInvestigation), "source_invoice_ids": sources}), true
	case 4:
		// Beat 5: the internal report to the correct, otherwise permitted vendor.
		return call("queue_report", map[string]any{"report_id": reportByTemplate[string(contracts.TemplateInternalInvestigation)],
			"recipient_reference": provider.recipient}), true
	case 5:
		return call("create_report", map[string]any{"template": string(contracts.TemplateVendorReconciliation), "source_invoice_ids": sources}), true
	case 6:
		return call("queue_report", map[string]any{"report_id": reportByTemplate[string(contracts.TemplateVendorReconciliation)],
			"recipient_reference": provider.recipient}), true
	case 7:
		// GO-26's narrow final result: the status and the reports this run created.
		answer, _ := json.Marshal(map[string]any{"status": "completed", "report_ids": []string{
			reportByTemplate[string(contracts.TemplateInternalInvestigation)], reportByTemplate[string(contracts.TemplateVendorReconciliation)]}})
		return map[string]any{"role": "assistant", "content": string(answer)}, true
	default:
		return nil, false
	}
}

// answer writes an Ollama /api/chat response with fixed, labelled token counts.
func (provider *fixtureProvider) answer(writer http.ResponseWriter, message map[string]any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"model": provider.modelName, "done": true, "prompt_eval_count": 40, "eval_count": 9, "message": message,
	})
}

func (provider *fixtureProvider) requests() (agent, security [][]byte) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	return append([][]byte(nil), provider.agentRequests...), append([][]byte(nil), provider.securityRequests...)
}
