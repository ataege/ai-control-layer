package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/budget/budgettest"
	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/testdb"
)

// fixedAccounting is a test stand-in for the active catalog's accounting projection.
type fixedAccounting struct{}

func (fixedAccounting) Active(context.Context) (config.AccountingCatalog, error) {
	return config.AccountingCatalog{RevisionID: 1, Settings: config.ModelAccounting{TokensTotal: 20000, AgentOutputTokens: 512,
		SecurityOutputTokens: 256, TemplateTokens: 1024, RequestTimeout: 20 * time.Second, AllowedModels: []string{"test-fixture"}}}, nil
}

func gatewaySnapshot(allowedModels []string, requestTimeoutSeconds, localMaxConcurrency int64) fixedCatalog {
	return fixedCatalog{snapshot: catalog.Snapshot{RevisionID: 1, Limits: catalog.Limits{AllowedModels: allowedModels,
		CallsTotal: 24, CallsAgent: 12, CallsSecurity: 12, TokensTotal: 20000, RequestTimeoutSeconds: requestTimeoutSeconds,
		LocalMaxConcurrency: localMaxConcurrency}}}
}

// providerDouble is a labelled local HTTP provider test double behind the real Ollama transport.
func providerDouble(t *testing.T, handler http.HandlerFunc) model.ChatProvider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	provider, err := model.NewOllama(model.Options{BaseURL: server.URL, Model: "test-fixture", Timeout: time.Minute,
		MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

const fixtureAnswer = `{"model":"test-fixture","done":true,"message":{"role":"assistant","content":"fixture"},"prompt_eval_count":10,"eval_count":2}`

func agentRequest() model.Request {
	return model.Request{Purpose: model.AgentPurpose, ContextTokens: contextTokens, Messages: []model.Message{{Role: "user", Content: "fixture"}}}
}

func TestModelGatewayRefusesModelsOutsideCatalogOrPassport(t *testing.T) {
	pool := testdb.Open(t)
	run := budgettest.OpenRun(t, pool, budgettest.Limits(20000))
	var hits atomic.Int64
	provider := providerDouble(t, func(writer http.ResponseWriter, request *http.Request) {
		hits.Add(1)
		fmt.Fprint(writer, fixtureAnswer)
	})
	ledger := budgettest.NewDispatchRecordingStore(pool)
	for name, testCase := range map[string]struct {
		modelName string
		catalog   []string
	}{
		"model removed from the catalog": {modelName: "test-fixture", catalog: []string{"another-model"}},
		"model the passport never named": {modelName: "second-model", catalog: []string{"test-fixture", "second-model"}},
	} {
		caller := NewCatalogAccountedCaller(provider, ledger, fixedAccounting{}, gatewaySnapshot(testCase.catalog, 20, 2), pool, testCase.modelName)
		if _, err := caller.Call(context.Background(), run.RunID, testdb.ID(t), agentRequest()); !errors.Is(err, ErrModelNotAllowed) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	snapshot, err := budget.NewPostgresStore(pool).Snapshot(context.Background(), run.RunID)
	if err != nil || hits.Load() != 0 || snapshot.Reserved != 0 || snapshot.Agent.Calls != 0 {
		t.Fatalf("a refused model dispatched or reserved: hits=%d %+v %v", hits.Load(), snapshot, err)
	}
}

func TestModelGatewayHoldsAThirdConcurrentRequestOverACapOfTwo(t *testing.T) {
	pool := testdb.Open(t)
	release := make(chan struct{})
	var inFlight, maximumInFlight, hits atomic.Int64
	provider := providerDouble(t, func(writer http.ResponseWriter, request *http.Request) {
		hits.Add(1)
		current := inFlight.Add(1)
		for {
			seen := maximumInFlight.Load()
			if current <= seen || maximumInFlight.CompareAndSwap(seen, current) {
				break
			}
		}
		<-release
		inFlight.Add(-1)
		fmt.Fprint(writer, fixtureAnswer)
	})
	caller := NewCatalogAccountedCaller(provider, budgettest.NewDispatchRecordingStore(pool), fixedAccounting{},
		gatewaySnapshot([]string{"test-fixture"}, 20, 2), pool, "test-fixture")
	// Three runs, so the per-run ledger slot is not what holds the third request back.
	runs := []budgettest.Run{budgettest.OpenRun(t, pool, budgettest.Limits(20000)), budgettest.OpenRun(t, pool, budgettest.Limits(20000)),
		budgettest.OpenRun(t, pool, budgettest.Limits(20000))}
	callIDs := []string{testdb.ID(t), testdb.ID(t), testdb.ID(t)}
	var waitGroup sync.WaitGroup
	failures := make(chan error, 3)
	for index, run := range runs {
		waitGroup.Add(1)
		go func(runID, callID string) {
			defer waitGroup.Done()
			if _, err := caller.Call(context.Background(), runID, callID, agentRequest()); err != nil {
				failures <- err
			}
		}(run.RunID, callIDs[index])
	}
	deadline := time.Now().Add(10 * time.Second)
	for inFlight.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	if hits.Load() != 2 {
		t.Fatalf("%d requests reached the provider with a cap of two", hits.Load())
	}
	close(release)
	waitGroup.Wait()
	close(failures)
	for err := range failures {
		t.Fatalf("a held request failed instead of waiting: %v", err)
	}
	if hits.Load() != 3 || maximumInFlight.Load() != 2 {
		t.Fatalf("hits %d, maximum in flight %d", hits.Load(), maximumInFlight.Load())
	}
}

func TestModelGatewayDeadlineRetainsTheReservationAndSlot(t *testing.T) {
	pool := testdb.Open(t)
	run := budgettest.OpenRun(t, pool, budgettest.Limits(20000))
	provider := providerDouble(t, func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
		case <-time.After(3 * time.Second):
		}
	})
	caller := NewCatalogAccountedCaller(provider, budgettest.NewDispatchRecordingStore(pool), fixedAccounting{},
		gatewaySnapshot([]string{"test-fixture"}, 1, 2), pool, "test-fixture")
	started := time.Now()
	result, err := caller.Call(context.Background(), run.RunID, testdb.ID(t), agentRequest())
	if err == nil || !result.UsageUnknown || time.Since(started) > 5*time.Second {
		t.Fatalf("the catalog deadline did not end the request: %+v %v after %v", result, err, time.Since(started))
	}
	snapshot, err := budget.NewPostgresStore(pool).Snapshot(context.Background(), run.RunID)
	if err != nil || snapshot.Reserved == 0 || snapshot.Agent.UnresolvedCalls != 1 || snapshot.CallsInFlight != 1 {
		t.Fatalf("the timed-out call's reservation and ledger slot were not retained: %+v %v", snapshot, err)
	}
	caller.local.mutex.Lock()
	held := caller.local.inUse
	caller.local.mutex.Unlock()
	if held != 1 {
		t.Fatalf("the process slot was released at once after a timeout (in use %d)", held)
	}
}

// otherRevisionAccounting reports an accounting projection of another revision than the snapshot.
type otherRevisionAccounting struct{}

func (otherRevisionAccounting) Active(ctx context.Context) (config.AccountingCatalog, error) {
	accounting, _ := fixedAccounting{}.Active(ctx)
	accounting.RevisionID = 2
	return accounting, nil
}

func TestModelGatewayRefusesACallAcrossTwoCatalogRevisions(t *testing.T) {
	pool := testdb.Open(t)
	run := budgettest.OpenRun(t, pool, budgettest.Limits(20000))
	var hits atomic.Int64
	provider := providerDouble(t, func(writer http.ResponseWriter, request *http.Request) {
		hits.Add(1)
		fmt.Fprint(writer, fixtureAnswer)
	})
	caller := NewCatalogAccountedCaller(provider, budgettest.NewDispatchRecordingStore(pool), otherRevisionAccounting{},
		gatewaySnapshot([]string{"test-fixture"}, 20, 2), pool, "test-fixture")
	if _, err := caller.Call(context.Background(), run.RunID, testdb.ID(t), agentRequest()); !errors.Is(err, catalog.ErrUnavailable) {
		t.Fatalf("mixed revisions: %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("a call across two catalog revisions was dispatched")
	}
}
