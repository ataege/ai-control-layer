package model

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"starter/services/gateway/internal/budget"
)

// accountingFixtureStore is a test-only ledger, not runtime persistence.
type accountingFixtureStore struct {
	mutex                     sync.Mutex
	limit, reserved, used     int64
	paused                    bool
	unknown                   bool
	reserveError, settleError error
	// markUnknownError makes marking a call as unknown fail, as a slow or lost database does.
	markUnknownError error
}

func (store *accountingFixtureStore) Reserve(_ context.Context, _, _, _ string, tokens int64) (budget.Reservation, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.reserveError != nil {
		return budget.Reservation{}, store.reserveError
	}
	if tokens > store.limit-store.reserved-store.used {
		return budget.Reservation{}, budget.ErrExhausted
	}
	store.reserved += tokens
	return budget.Reservation{Tokens: tokens}, nil
}
func (store *accountingFixtureStore) MarkUnknown(ctx context.Context, _, _ string) error {
	if store.markUnknownError != nil {
		return store.markUnknownError
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.unknown = true
	return nil
}
func (store *accountingFixtureStore) Settle(_ context.Context, _, _ string, input, output int64) (budget.Settlement, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.settleError != nil {
		return budget.Settlement{}, store.settleError
	}
	actual := input + output
	reserved := store.reserved
	store.reserved = 0
	store.used += actual
	store.paused = actual > reserved
	refund := int64(0)
	if actual < reserved {
		refund = reserved - actual
	}
	return budget.Settlement{ActualTokens: actual, RefundedTokens: refund, Paused: store.paused}, nil
}
func (store *accountingFixtureStore) Snapshot(context.Context, string) (budget.Snapshot, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	return budget.Snapshot{Limit: store.limit, Used: store.used, Reserved: store.reserved, Paused: store.paused}, nil
}

type accountingProviderFunc func(context.Context, Request) (Result, error)

func (provider accountingProviderFunc) Chat(ctx context.Context, request Request) (Result, error) {
	return provider(ctx, request)
}
func accountedFixtureRequest() Request {
	return Request{Purpose: AgentPurpose, Messages: []Message{{Role: "user", Content: "synthetic fixture"}}, ContextTokens: 4096}
}
func tokenCount(value int64) *int64 { return &value }
func fixtureResult(input, output int64) Result {
	return Result{Message: Message{Role: "assistant", Content: "fixture response"}, Usage: Usage{InputTokens: tokenCount(input), OutputTokens: tokenCount(output)}}
}
func fixtureAccountedCaller(t *testing.T, store *accountingFixtureStore, provider accountingProviderFunc) *AccountedCaller {
	t.Helper()
	caller, err := NewAccountedCaller(provider, store, DefaultAccountingSettings())
	if err != nil {
		t.Fatal(err)
	}
	return caller
}
func TestAccountedCallReservesBeforeDispatchAndRefunds(t *testing.T) {
	store := &accountingFixtureStore{limit: 10000}
	caller := fixtureAccountedCaller(t, store, func(_ context.Context, request Request) (Result, error) {
		if store.reserved == 0 {
			t.Fatal("dispatched without reservation")
		}
		if request.OutputTokens != 512 || request.Think == nil || *request.Think {
			t.Fatal("trusted generation options missing")
		}
		return fixtureResult(50, 20), nil
	})
	result, err := caller.Call(context.Background(), "run", "call", accountedFixtureRequest())
	if err != nil || result.Settlement == nil || result.Settlement.ActualTokens != 70 || result.Settlement.RefundedTokens != result.ReservationTokens-70 || store.used != 70 || store.reserved != 0 {
		t.Fatalf("bad settlement: %+v %v", result, err)
	}
}
func TestAccountedCallRetainsUncertainReservation(t *testing.T) {
	for _, scenario := range []string{"missing-input", "missing-output", "timeout", "cancel", "negative", "overflow"} {
		t.Run(scenario, func(t *testing.T) {
			store := &accountingFixtureStore{limit: 10000}
			calls := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			caller := fixtureAccountedCaller(t, store, func(context.Context, Request) (Result, error) {
				calls++
				result := fixtureResult(2, 3)
				switch scenario {
				case "missing-input":
					result.Usage.InputTokens = nil
				case "missing-output":
					result.Usage.OutputTokens = nil
				case "timeout":
					return result, ErrTimeout
				case "cancel":
					cancel()
					return result, context.Canceled
				case "negative":
					result.Usage.InputTokens = tokenCount(-1)
				case "overflow":
					result.Usage.InputTokens = tokenCount(math.MaxInt64)
				}
				return result, nil
			})
			result, err := caller.Call(ctx, "run", "call", accountedFixtureRequest())
			if err == nil || !result.UsageUnknown || !store.unknown || store.reserved != result.ReservationTokens || store.used != 0 || calls != 1 || result.Provider.Message.Content != "" {
				t.Fatalf("uncertain usage released: %+v %v", result, err)
			}
		})
	}
}
func TestAccountedCallOverspendChargesFullUsageAndWithholdsMessage(t *testing.T) {
	store := &accountingFixtureStore{limit: 10000}
	caller := fixtureAccountedCaller(t, store, func(context.Context, Request) (Result, error) { return fixtureResult(5000, 100), nil })
	result, err := caller.Call(context.Background(), "run", "call", accountedFixtureRequest())
	if !errors.Is(err, ErrOverspend) || store.used != 5100 || !store.paused || result.Provider.Message.Content != "" {
		t.Fatalf("overspend hidden: %+v %v", result, err)
	}
}
func TestAccountedCallExhaustionAndSettlementFailure(t *testing.T) {
	store := &accountingFixtureStore{limit: 1}
	calls := 0
	caller := fixtureAccountedCaller(t, store, func(context.Context, Request) (Result, error) { calls++; return fixtureResult(5, 5), nil })
	if _, err := caller.Call(context.Background(), "run", "call", accountedFixtureRequest()); !errors.Is(err, budget.ErrExhausted) || calls != 0 {
		t.Fatal("exhausted call dispatched")
	}
	store.limit = 10000
	store.settleError = errors.New("private database details")
	result, err := caller.Call(context.Background(), "run", "call", accountedFixtureRequest())
	// A failed settlement holds the reservation as unknown usage, never a success or a silent loss.
	if !errors.Is(err, ErrUsageUnknown) || !result.UsageUnknown || !store.unknown || result.Provider.Message.Content != "" || store.reserved == 0 {
		t.Fatalf("failed settlement: err %v, usage unknown %v, marked %v, reserved %d", err, result.UsageUnknown, store.unknown, store.reserved)
	}
}
func TestEstimateIncludesEveryInputAndRejectsOverflow(t *testing.T) {
	request := accountedFixtureRequest()
	settings := DefaultAccountingSettings()
	base, err := EstimateReservation(request, settings)
	if err != nil {
		t.Fatal(err)
	}
	request.Messages = append(request.Messages, Message{Role: "system", Content: "日本語 system"}, Message{Role: "tool", Content: "tool result"})
	request.Tools = []Tool{{Type: "function", Function: FunctionDefinition{Name: "read", Parameters: []byte(`{"type":"object","properties":{"record":{"type":"string"}}}`)}}}
	request.Format = []byte(`{"type":"object","properties":{"result":{"type":"string"}}}`)
	expanded, err := EstimateReservation(request, settings)
	if err != nil || expanded <= base {
		t.Fatal("input omitted from estimate")
	}
	settings.TemplateTokens = math.MaxInt64
	if _, err = EstimateReservation(request, settings); err == nil {
		t.Fatal("overflow accepted")
	}
	settings = DefaultAccountingSettings()
	settings.AgentOutputTokens = 0
	if _, err = EstimateReservation(request, settings); err == nil {
		t.Fatal("nonpositive limit accepted")
	}
}
func TestSharedAllowancePreventsConcurrentOversubscription(t *testing.T) {
	request := accountedFixtureRequest()
	estimate, _ := EstimateReservation(request, DefaultAccountingSettings())
	store := &accountingFixtureStore{limit: estimate}
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	caller := fixtureAccountedCaller(t, store, func(context.Context, Request) (Result, error) {
		entered <- struct{}{}
		<-release
		return fixtureResult(1, 1), nil
	})
	first := make(chan error, 1)
	go func() { _, err := caller.Call(context.Background(), "run", "first", request); first <- err }()
	<-entered
	if _, err := caller.Call(context.Background(), "run", "second", request); !errors.Is(err, budget.ErrExhausted) {
		t.Fatal("concurrent oversubscription accepted")
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
}

func TestAccountedPreflightRejectsInvalidInputsBeforeReservation(t *testing.T) {
	for _, scenario := range []string{"invalid-format", "invalid-tool", "oversized-native-body"} {
		t.Run(scenario, func(t *testing.T) {
			store := &accountingFixtureStore{limit: 10000}
			provider, err := NewOllama(Options{BaseURL: "http://127.0.0.1:1", Model: "fixture:tag", Timeout: time.Second, MaxRequestBytes: 32, MaxResponseBytes: 1024})
			if err != nil {
				t.Fatal(err)
			}
			caller, err := NewAccountedCaller(provider, store, DefaultAccountingSettings())
			if err != nil {
				t.Fatal(err)
			}
			request := accountedFixtureRequest()
			switch scenario {
			case "invalid-format":
				request.Format = []byte(`42`)
			case "invalid-tool":
				request.Tools = []Tool{{Type: "invalid", Function: FunctionDefinition{Name: "read", Parameters: []byte(`{}`)}}}
			}
			result, err := caller.Call(context.Background(), "run", "call", request)
			if !errors.Is(err, ErrRequest) || store.reserved != 0 || result.ReservationTokens != 0 || store.unknown {
				t.Fatalf("local invalid input reserved or dispatched: %+v %v", result, err)
			}
		})
	}
}

func TestReconcileRejectsNilContext(t *testing.T) {
	caller := fixtureAccountedCaller(t, &accountingFixtureStore{}, func(context.Context, Request) (Result, error) { return Result{}, nil })
	if _, err := caller.Reconcile(nil, "run", "call", 1, 1); !errors.Is(err, ErrRequest) {
		t.Fatal("nil context accepted")
	}
}

// A dispatched call whose unknown usage could not be persisted is still an unknown outcome: the
// error says so (ErrUsageUnknown) next to the accounting failure, so the run pauses for attention
// instead of failing as if no request had been sent.
func TestUnpersistedUnknownUsageIsStillUnknownUsage(t *testing.T) {
	for _, scenario := range []string{"provider timeout", "settlement fails"} {
		t.Run(scenario, func(t *testing.T) {
			store := &accountingFixtureStore{limit: 10000, markUnknownError: errors.New("private database details")}
			if scenario == "settlement fails" {
				store.settleError = errors.New("private database details")
			}
			caller := fixtureAccountedCaller(t, store, func(context.Context, Request) (Result, error) {
				if scenario == "provider timeout" {
					return Result{}, ErrTimeout
				}
				return fixtureResult(2, 3), nil
			})
			result, err := caller.Call(context.Background(), "run", "call", accountedFixtureRequest())
			if !errors.Is(err, ErrUsageUnknown) || !errors.Is(err, ErrAccounting) || !result.UsageUnknown || result.Provider.Message.Content != "" {
				t.Fatalf("err %v, usage unknown %v", err, result.UsageUnknown)
			}
			if store.reserved == 0 {
				t.Fatal("the reservation was released although the usage is unknown")
			}
		})
	}
}
