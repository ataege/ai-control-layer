package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"starter/services/gateway/internal/budget"
)

func postgresAccountingStore(t *testing.T) (*budget.PostgresStore, *pgxpool.Pool, string) {
	t.Helper()
	databaseURL := os.Getenv("GATEWAY_TEST_DATABASE_URL")
	if databaseURL == "" && os.Getenv("POSTGRES_USER") != "" && os.Getenv("POSTGRES_PASSWORD") != "" && os.Getenv("POSTGRES_DB") != "" {
		host := os.Getenv("POSTGRES_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("POSTGRES_PORT")
		if port == "" {
			port = "5432"
		}
		address := url.URL{Scheme: "postgresql", User: url.UserPassword(os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD")), Host: net.JoinHostPort(host, port), Path: "/" + os.Getenv("POSTGRES_DB")}
		databaseURL = address.String()
	}
	if databaseURL == "" {
		if os.Getenv("TEST_DATABASE_REQUIRED") == "1" {
			t.Fatal("PostgreSQL accounting tests required but configuration missing")
		}
		t.Skip("PostgreSQL accounting test configuration unset")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal("invalid PostgreSQL test configuration")
	}
	t.Cleanup(pool.Close)
	store := budget.NewPostgresStore(pool)
	id := fmt.Sprintf("model-accounting-test-%d", time.Now().UnixNano())
	if err := store.CreateRun(context.Background(), id, 20000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM runtime.model_token_reservations WHERE run_id=$1", id); err != nil {
			t.Error("could not clean test reservations")
		}
		if _, err := pool.Exec(context.Background(), "DELETE FROM runtime.model_token_budgets WHERE run_id=$1", id); err != nil {
			t.Error("could not clean test budget")
		}
	})
	return store, pool, id
}
func postgresAccountingCaller(t *testing.T, store budget.Store, handler http.HandlerFunc, timeout time.Duration) (*AccountedCaller, *atomic.Int64) {
	t.Helper()
	hits := new(atomic.Int64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); handler(w, r) }))
	t.Cleanup(server.Close)
	provider, err := NewOllama(Options{BaseURL: server.URL, Model: "test-fixture", Timeout: timeout, MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	caller, err := NewAccountedCaller(provider, store, DefaultAccountingSettings())
	if err != nil {
		t.Fatal(err)
	}
	return caller, hits
}
func postgresAccountingRequest(purpose Purpose) Request {
	return Request{Purpose: purpose, ContextTokens: 8192, Messages: []Message{{Role: "system", Content: "synthetic accounting fixture"}, {Role: "user", Content: "hello"}}}
}
func TestPostgresOllamaMeasuredSettlement(t *testing.T) {
	store, _, id := postgresAccountingStore(t)
	caller, hits := postgresAccountingCaller(t, store, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"model":"test-fixture","done":true,"message":{"role":"assistant","content":"ok"},"prompt_eval_count":200,"eval_count":50}`)
	}, time.Second)
	result, err := caller.Call(context.Background(), id, "normal", postgresAccountingRequest(AgentPurpose))
	if err != nil {
		t.Fatal(err)
	}
	if result.Settlement == nil || result.Settlement.ActualTokens != 250 || result.Settlement.RefundedTokens != result.ReservationTokens-250 {
		t.Fatalf("%+v", result)
	}
	snapshot, err := store.Snapshot(context.Background(), id)
	if err != nil || snapshot.Used != 250 || snapshot.Reserved != 0 || hits.Load() != 1 {
		t.Fatalf("%+v hits=%d %v", snapshot, hits.Load(), err)
	}
}
func TestPostgresOllamaMissingCounterRetained(t *testing.T) {
	store, pool, id := postgresAccountingStore(t)
	caller, _ := postgresAccountingCaller(t, store, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"model":"test-fixture","done":true,"message":{"role":"assistant","content":"ok"},"prompt_eval_count":200}`)
	}, time.Second)
	result, err := caller.Call(context.Background(), id, "missing", postgresAccountingRequest(SecurityPurpose))
	if !errors.Is(err, ErrUsageUnknown) || !result.UsageUnknown {
		t.Fatalf("%+v %v", result, err)
	}
	snapshot, err := store.Snapshot(context.Background(), id)
	if err != nil || snapshot.Reserved != result.ReservationTokens || snapshot.Used != 0 {
		t.Fatalf("%+v %v", snapshot, err)
	}
	var status string
	var input, output, actual *int64
	if err := pool.QueryRow(context.Background(), "SELECT status,input_tokens,output_tokens,actual_tokens FROM runtime.model_token_reservations WHERE run_id=$1 AND call_id='missing'", id).Scan(&status, &input, &output, &actual); err != nil {
		t.Fatal("could not inspect test reservation")
	}
	if status != "usage_unknown" || input != nil || output != nil || actual != nil {
		t.Fatalf("unexpected persisted usage: %s %v %v %v", status, input, output, actual)
	}
}
func TestPostgresOllamaTimeoutNoRedispatch(t *testing.T) {
	store, _, id := postgresAccountingStore(t)
	caller, hits := postgresAccountingCaller(t, store, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
		case <-timer.C:
		}
	}, 10*time.Millisecond)
	request := postgresAccountingRequest(AgentPurpose)
	result, err := caller.Call(context.Background(), id, "timeout", request)
	if !errors.Is(err, ErrTimeout) || !result.UsageUnknown {
		t.Fatalf("%+v %v", result, err)
	}
	if _, err := caller.Call(context.Background(), id, "timeout", request); !errors.Is(err, budget.ErrDuplicate) {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(context.Background(), id)
	if err != nil || snapshot.Reserved != result.ReservationTokens || snapshot.Used != 0 || hits.Load() != 1 {
		t.Fatalf("%+v hits=%d %v", snapshot, hits.Load(), err)
	}
}
func TestPostgresOllamaInsufficientBudgetNoHTTP(t *testing.T) {
	store, _, id := postgresAccountingStore(t)
	if _, err := store.Reserve(context.Background(), id, "held", "agent", 20000); err != nil {
		t.Fatal(err)
	}
	caller, hits := postgresAccountingCaller(t, store, func(w http.ResponseWriter, r *http.Request) { t.Error("provider dispatched without allowance") }, time.Second)
	if _, err := caller.Call(context.Background(), id, "denied", postgresAccountingRequest(AgentPurpose)); !errors.Is(err, budget.ErrExhausted) {
		t.Fatal(err)
	}
	if hits.Load() != 0 {
		t.Fatal("unexpected dispatch")
	}
}
func TestPostgresOllamaConcurrentSharedAllowance(t *testing.T) {
	store, _, id := postgresAccountingStore(t)
	caller, hits := postgresAccountingCaller(t, store, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"model":"test-fixture","done":true,"message":{"role":"assistant","content":"fixture"}}`)
	}, time.Second)
	var wg sync.WaitGroup
	results := make(chan AccountedResult, 30)
	failures := make(chan error, 30)
	for i := range 30 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			purpose := AgentPurpose
			if i%2 == 1 {
				purpose = SecurityPurpose
			}
			result, err := caller.Call(context.Background(), id, fmt.Sprintf("concurrent-%d", i), postgresAccountingRequest(purpose))
			if errors.Is(err, ErrUsageUnknown) {
				results <- result
			} else {
				failures <- err
			}
		}(i)
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if !errors.Is(err, budget.ErrExhausted) {
			t.Fatal(err)
		}
	}
	var held int64
	var succeeded int64
	for result := range results {
		held += result.ReservationTokens
		succeeded++
	}
	snapshot, err := store.Snapshot(context.Background(), id)
	if err != nil || snapshot.Reserved != held || held > 20000 || snapshot.Used != 0 || hits.Load() != succeeded || succeeded == 0 {
		t.Fatalf("%+v held=%d successful=%d hits=%d %v", snapshot, held, succeeded, hits.Load(), err)
	}
}
func TestPostgresOllamaOverspendPausesFullCount(t *testing.T) {
	store, _, id := postgresAccountingStore(t)
	caller, hits := postgresAccountingCaller(t, store, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"model":"test-fixture","done":true,"message":{"role":"assistant","content":"fixture"},"prompt_eval_count":24000,"eval_count":500}`)
	}, time.Second)
	result, err := caller.Call(context.Background(), id, "over", postgresAccountingRequest(AgentPurpose))
	if !errors.Is(err, ErrOverspend) || result.Settlement == nil || result.Settlement.ActualTokens != 24500 {
		t.Fatalf("%+v %v", result, err)
	}
	snapshot, err := store.Snapshot(context.Background(), id)
	if err != nil || snapshot.Used != 24500 || snapshot.Reserved != 0 || !snapshot.Paused {
		t.Fatalf("%+v %v", snapshot, err)
	}
	if _, err := caller.Call(context.Background(), id, "next", postgresAccountingRequest(SecurityPurpose)); !errors.Is(err, budget.ErrPaused) {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatal("paused run dispatched")
	}
}
