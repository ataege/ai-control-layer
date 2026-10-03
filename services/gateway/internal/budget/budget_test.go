package budget

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestValidationAndUnavailableFailClosed(t *testing.T) {
	s := NewPostgresStore(nil)
	ctx := context.Background()
	if _, err := s.Reserve(ctx, "run", "call", "agent", 100); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	for _, purpose := range []string{"", "guard", "other"} {
		if _, err := s.Reserve(ctx, "run", "call", purpose, 100); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := s.Settle(ctx, "run", "call", math.MaxInt64, 1); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Settle(ctx, "run", "call", -1, 1); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := s.CreateRun(ctx, "run", 0); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
func databaseStore(t *testing.T) (*PostgresStore, string) {
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
			t.Fatal("PostgreSQL ledger tests required but database configuration is missing")
		}
		t.Skip("PostgreSQL test configuration unset: ledger integration test not run")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	id := fmt.Sprintf("budget-test-%d", time.Now().UnixNano())
	s := NewPostgresStore(pool)
	if err = s.CreateRun(context.Background(), id, 20000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), "DELETE FROM runtime.model_token_reservations WHERE run_id=$1", id)
		if err != nil {
			t.Error(err)
		}
		_, err = pool.Exec(context.Background(), "DELETE FROM runtime.model_token_budgets WHERE run_id=$1", id)
		if err != nil {
			t.Error(err)
		}
	})
	return s, id
}
func TestPostgresSettlementDurabilityAndUnknown(t *testing.T) {
	s, id := databaseStore(t)
	ctx := context.Background()
	if _, err := s.Reserve(ctx, id, "normal", "agent", 2000); err != nil {
		t.Fatal(err)
	}
	got, err := s.Settle(ctx, id, "normal", 700, 50)
	if err != nil || got.ActualTokens != 750 || got.RefundedTokens != 1250 {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err = s.Reserve(ctx, id, "late", "security", 2000); err != nil {
		t.Fatal(err)
	}
	if err = s.MarkUnknown(ctx, id, "late"); err != nil {
		t.Fatal(err)
	}
	// A fresh store reads persisted counters, rather than process-local state.
	restarted := NewPostgresStore(s.pool)
	snap, err := restarted.Snapshot(ctx, id)
	if err != nil || snap.Reserved != 2000 || snap.Used != 750 {
		t.Fatalf("%+v %v", snap, err)
	}
	var state string
	if err = s.pool.QueryRow(ctx, "SELECT status FROM runtime.model_token_reservations WHERE run_id=$1 AND call_id='late'", id).Scan(&state); err != nil || state != "usage_unknown" {
		t.Fatalf("%s %v", state, err)
	}
	got, err = restarted.Settle(ctx, id, "late", 300, 100)
	if err != nil || got.RefundedTokens != 1600 {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = s.Settle(ctx, id, "late", 300, 100)
	if err != nil || !got.AlreadySettled {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err = s.Settle(ctx, id, "late", 301, 100); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err = s.MarkUnknown(ctx, id, "late"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reserve(ctx, id, "late", "security", 2000); !errors.Is(err, ErrDuplicate) {
		t.Fatal(err)
	}
	snap, err = s.Snapshot(ctx, id)
	if err != nil || snap.Reserved != 0 || snap.Used != 1150 {
		t.Fatalf("%+v %v", snap, err)
	}
}
func TestPostgresConcurrentSharedBudget(t *testing.T) {
	s, id := databaseStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan error, 40)
	for i := range 40 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			purpose := "agent"
			if i%2 == 1 {
				purpose = "security"
			}
			_, err := s.Reserve(ctx, id, fmt.Sprintf("call-%d", i), purpose, 1000)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrExhausted) {
			t.Fatal(err)
		}
	}
	snap, err := s.Snapshot(ctx, id)
	if err != nil || success != 20 || snap.Reserved != 20000 {
		t.Fatalf("success=%d %+v %v", success, snap, err)
	}
}
func TestPostgresOverrunPausesWithoutClipping(t *testing.T) {
	s, id := databaseStore(t)
	ctx := context.Background()
	if _, err := s.Reserve(ctx, id, "call", "agent", 100); err != nil {
		t.Fatal(err)
	}
	got, err := s.Settle(ctx, id, "call", 150, 75)
	if err != nil || !got.Paused || got.ActualTokens != 225 {
		t.Fatalf("%+v %v", got, err)
	}
	snap, err := s.Snapshot(ctx, id)
	if err != nil || snap.Used != 225 || snap.Reserved != 0 || !snap.Paused {
		t.Fatalf("%+v %v", snap, err)
	}
	if _, err = s.Reserve(ctx, id, "next", "security", 1); !errors.Is(err, ErrPaused) {
		t.Fatal(err)
	}
}

func TestPostgresConcurrentLateSettlementOnce(t *testing.T) {
	s, id := databaseStore(t)
	ctx := context.Background()
	if _, err := s.Reserve(ctx, id, "late", "agent", 2000); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkUnknown(ctx, id, "late"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan Settlement, 12)
	failures := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := s.Settle(ctx, id, "late", 700, 50)
			if err != nil {
				failures <- err
				return
			}
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	first := 0
	for result := range results {
		if !result.AlreadySettled {
			first++
		}
	}
	snap, err := s.Snapshot(ctx, id)
	if err != nil || first != 1 || snap.Used != 750 || snap.Reserved != 0 {
		t.Fatalf("first=%d %+v %v", first, snap, err)
	}
}

func TestNilContextRejected(t *testing.T) {
	s := NewPostgresStore(nil)
	if err := s.CreateRun(nil, "run", 100); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(nil, "run"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Reserve(nil, "run", "call", "agent", 100); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := s.MarkUnknown(nil, "run", "call"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Settle(nil, "run", "call", 1, 1); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestPostgresAggregateOverflowPauses(t *testing.T) {
	s, id := databaseStore(t)
	ctx := context.Background()
	if _, err := s.Reserve(ctx, id, "prior", "agent", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Settle(ctx, id, "prior", 1, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(ctx, id, "huge", "security", 100); err != nil {
		t.Fatal(err)
	}
	result, err := s.Settle(ctx, id, "huge", math.MaxInt64, 0)
	if !errors.Is(err, ErrInvalid) || !result.Paused || result.ActualTokens != math.MaxInt64 {
		t.Fatalf("%+v %v", result, err)
	}
	snap, err := s.Snapshot(ctx, id)
	if err != nil || !snap.Paused || snap.Used != 1 || snap.Reserved != 100 {
		t.Fatalf("%+v %v", snap, err)
	}
	if _, err := s.Reserve(ctx, id, "next", "agent", 1); !errors.Is(err, ErrPaused) {
		t.Fatal(err)
	}
	var status string
	if err := s.pool.QueryRow(ctx, "SELECT status FROM runtime.model_token_reservations WHERE run_id=$1 AND call_id='huge'", id).Scan(&status); err != nil || status != "usage_unknown" {
		t.Fatalf("%s %v", status, err)
	}
}
