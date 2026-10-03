package model

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"starter/services/gateway/internal/budget"
)

var (
	ErrAccounting   = errors.New("model accounting failed")
	ErrUsageUnknown = errors.New("model usage unknown")
	ErrOverspend    = errors.New("model reservation exceeded")
)

type ChatProvider interface {
	Chat(context.Context, Request) (Result, error)
}

type AccountingSettings struct {
	AgentOutputTokens    int
	SecurityOutputTokens int
	TemplateTokens       int64
}

func DefaultAccountingSettings() AccountingSettings {
	return AccountingSettings{AgentOutputTokens: 512, SecurityOutputTokens: 256, TemplateTokens: 1024}
}

func validAccountingSettings(settings AccountingSettings) bool {
	return settings.AgentOutputTokens > 0 && settings.SecurityOutputTokens > 0 && settings.TemplateTokens > 0
}

// EstimateReservation deliberately counts UTF-8 bytes, not claimed tokenizer
// tokens, plus a trusted template allowance and the maximum generated output.
func EstimateReservation(request Request, settings AccountingSettings) (int64, error) {
	if !validAccountingSettings(settings) {
		return 0, ErrRequest
	}
	output, err := outputAllowance(request.Purpose, settings)
	if err != nil {
		return 0, err
	}
	envelope := struct {
		Messages []Message       `json:"messages"`
		Tools    []Tool          `json:"tools,omitempty"`
		Format   json.RawMessage `json:"format,omitempty"`
	}{request.Messages, request.Tools, request.Format}
	body, err := json.Marshal(envelope)
	if err != nil {
		return 0, ErrRequest
	}
	bytes := int64(len(body))
	if bytes > math.MaxInt64-settings.TemplateTokens || bytes+settings.TemplateTokens > math.MaxInt64-int64(output) {
		return 0, ErrRequest
	}
	return bytes + settings.TemplateTokens + int64(output), nil
}

func outputAllowance(purpose Purpose, settings AccountingSettings) (int, error) {
	switch purpose {
	case AgentPurpose:
		return settings.AgentOutputTokens, nil
	case SecurityPurpose:
		return settings.SecurityOutputTokens, nil
	default:
		return 0, ErrRequest
	}
}

type AccountedResult struct {
	Provider          Result
	ReservationTokens int64
	Settlement        *budget.Settlement
	UsageUnknown      bool
}

type AccountedCaller struct {
	provider ChatProvider
	store    budget.Store
	settings AccountingSettings
}

func NewAccountedCaller(provider ChatProvider, store budget.Store, settings AccountingSettings) (*AccountedCaller, error) {
	if provider == nil || store == nil || !validAccountingSettings(settings) {
		return nil, ErrConfiguration
	}
	return &AccountedCaller{provider: provider, store: store, settings: settings}, nil
}

func (caller *AccountedCaller) Call(ctx context.Context, runID, callID string, request Request) (AccountedResult, error) {
	var result AccountedResult
	if ctx == nil || runID == "" || callID == "" || len(request.Messages) == 0 {
		return result, ErrRequest
	}
	output, err := outputAllowance(request.Purpose, caller.settings)
	if err != nil || request.ContextTokens < output {
		return result, ErrRequest
	}
	request.OutputTokens = output
	thinking := false
	request.Think = &thinking
	if err := validateRequest(request); err != nil {
		return result, err
	}
	if validator, ok := caller.provider.(interface{ ValidateRequest(Request) error }); ok {
		if err := validator.ValidateRequest(request); err != nil {
			return result, ErrRequest
		}
	}
	reservation, err := EstimateReservation(request, caller.settings)
	if err != nil {
		return result, err
	}
	result.ReservationTokens = reservation
	granted, err := caller.store.Reserve(ctx, runID, callID, string(request.Purpose), reservation)
	if err != nil {
		return result, safeBudgetError(err)
	}
	// The ledger is the authority for request time: the provider request ends at its deadline.
	dispatchContext := ctx
	if granted.RequestTimeout > 0 {
		var cancelDispatch context.CancelFunc
		dispatchContext, cancelDispatch = context.WithTimeout(ctx, granted.RequestTimeout)
		defer cancelDispatch()
	}
	result.Provider, err = caller.provider.Chat(dispatchContext, request)
	input, generated := result.Provider.Usage.InputTokens, result.Provider.Usage.OutputTokens
	if err != nil || input == nil || generated == nil || *input < 0 || *generated < 0 || *input > math.MaxInt64-*generated {
		result.Provider.Message = Message{}
		result.UsageUnknown = true
		// Cancellation must not prevent persisting an already-dispatched call's
		// uncertain accounting state. The reservation remains held either way.
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if caller.store.MarkUnknown(cleanupContext, runID, callID) != nil {
			return result, ErrAccounting
		}
		if errors.Is(err, context.Canceled) {
			return result, context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return result, context.DeadlineExceeded
		}
		if errors.Is(err, ErrTimeout) {
			return result, ErrTimeout
		}
		return result, ErrUsageUnknown
	}
	settlement, err := caller.store.Settle(ctx, runID, callID, *input, *generated)
	if err != nil {
		result.Provider.Message = Message{}
		return result, safeBudgetError(err)
	}
	result.Settlement = &settlement
	if settlement.Paused || settlement.ActualTokens > reservation {
		result.Provider.Message = Message{}
		return result, ErrOverspend
	}
	return result, nil
}

// Reconcile accepts trusted late counters locally; the persistent store
// enforces one settlement and rejects conflicting repeated usage.
func (caller *AccountedCaller) Reconcile(ctx context.Context, runID, callID string, input, output int64) (budget.Settlement, error) {
	if ctx == nil {
		return budget.Settlement{}, ErrRequest
	}
	settlement, err := caller.store.Settle(ctx, runID, callID, input, output)
	if err != nil {
		return settlement, safeBudgetError(err)
	}
	if settlement.Paused {
		return settlement, ErrOverspend
	}
	return settlement, nil
}

func safeBudgetError(err error) error {
	for _, known := range []error{budget.ErrInvalid, budget.ErrNotFound, budget.ErrExhausted, budget.ErrPaused, budget.ErrDuplicate,
		budget.ErrConflict, budget.ErrConcurrencyLimit, budget.ErrUnavailable} {
		if errors.Is(err, known) {
			return known
		}
	}
	return ErrAccounting
}
