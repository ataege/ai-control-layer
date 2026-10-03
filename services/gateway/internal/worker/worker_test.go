package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"starter/services/gateway/internal/testdb"
)

func testLogger(output *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestNewRejectsUnusableOptions(t *testing.T) {
	handler := HandlerFunc(func(context.Context, Job) (Outcome, error) { return Completed(), nil })
	logger := testLogger(&bytes.Buffer{})
	store := NewJobStore(nil)
	valid := Options{WorkerID: "worker-a", Kinds: []string{"kind"}, Logger: logger}
	if _, err := New(store, handler, valid); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}
	for name, options := range map[string]Options{
		"no logger":                 {WorkerID: "worker-a", Kinds: []string{"kind"}},
		"no kinds":                  {WorkerID: "worker-a", Logger: logger},
		"blank kind":                {WorkerID: "worker-a", Kinds: []string{""}, Logger: logger},
		"unsafe worker id":          {WorkerID: "a b", Kinds: []string{"kind"}, Logger: logger},
		"renewal not before expiry": {WorkerID: "worker-a", Kinds: []string{"kind"}, Logger: logger, LeaseDuration: time.Second, RenewInterval: time.Second},
		"lease too short":           {WorkerID: "worker-a", Kinds: []string{"kind"}, Logger: logger, LeaseDuration: time.Millisecond},
	} {
		if _, err := New(store, handler, options); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := New(store, nil, valid); !errors.Is(err, ErrInvalid) {
		t.Errorf("nil handler: %v", err)
	}
	if _, err := New(nil, handler, valid); !errors.Is(err, ErrInvalid) {
		t.Errorf("nil store: %v", err)
	}
}

func TestWorkerHoldsNoLockOrTransactionWhileTheHandlerRuns(t *testing.T) {
	pool := testdb.Open(t)
	observer := testdb.Open(t)
	fixture := newJobFixture(t, pool, "", 0)

	// Both checks are scoped to this test, so concurrent test packages cannot disturb them.
	rowLockedDuringHandler := true
	connectionsHeldDuringHandler := int32(-1)
	handler := HandlerFunc(func(ctx context.Context, job Job) (Outcome, error) {
		// Stands in for a model or tool request: the claim must already be committed.
		// pgxpool destroys a connection released inside a transaction, so zero acquired
		// connections means the worker holds no open transaction.
		connectionsHeldDuringHandler = pool.Stat().AcquiredConns()
		transaction, err := observer.Begin(ctx)
		if err != nil {
			return Outcome{}, err
		}
		defer func() { _ = transaction.Rollback(ctx) }()
		// NOWAIT fails at once if any transaction still holds the job row.
		var lockedID string
		err = transaction.QueryRow(ctx, "SELECT id::text FROM runtime.jobs WHERE id = $1 FOR UPDATE NOWAIT", job.ID).Scan(&lockedID)
		rowLockedDuringHandler = err != nil
		return Completed(), nil
	})
	worker, err := New(NewJobStore(pool), handler, Options{
		WorkerID: "worker-a", Kinds: []string{fixture.kind}, Logger: testLogger(&bytes.Buffer{}),
		LeaseDuration: 10 * time.Second, RenewInterval: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	processed, err := worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("run once: %v %v", processed, err)
	}
	if rowLockedDuringHandler {
		t.Fatal("the job row was still locked while the handler ran")
	}
	if connectionsHeldDuringHandler != 0 {
		t.Fatalf("the worker held %d connections while the handler ran", connectionsHeldDuringHandler)
	}
	if status, leaseOwner, _ := fixture.jobRow(t); status != string(JobCompleted) || leaseOwner != nil {
		t.Fatalf("job after completion: %s %v", status, leaseOwner)
	}
}

func TestWorkerRenewsTheLeaseOfALongHandler(t *testing.T) {
	pool := testdb.Open(t)
	fixture := newJobFixture(t, pool, "", 0)
	store := NewJobStore(pool)

	stolen := false
	handler := HandlerFunc(func(ctx context.Context, job Job) (Outcome, error) {
		// Outlive the original lease several times over; renewal must keep the job ours.
		deadline := time.Now().Add(1200 * time.Millisecond)
		for time.Now().Before(deadline) {
			if _, claimed, err := store.Claim(ctx, "worker-b", []string{fixture.kind}, time.Second); err != nil || claimed {
				stolen = claimed
				return Outcome{}, errors.New("renewal check failed")
			}
			time.Sleep(100 * time.Millisecond)
		}
		return Requeue(0), nil
	})
	worker, err := New(store, handler, Options{
		WorkerID: "worker-a", Kinds: []string{fixture.kind}, Logger: testLogger(&bytes.Buffer{}),
		LeaseDuration: 300 * time.Millisecond, RenewInterval: 75 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if processed, err := worker.RunOnce(context.Background()); err != nil || !processed || stolen {
		t.Fatalf("run once: processed %v, err %v, stolen %v", processed, err, stolen)
	}
	// Requeue returns the job to the queue with no lease.
	if status, leaseOwner, claims := fixture.jobRow(t); status != string(JobQueued) || leaseOwner != nil || claims != 1 {
		t.Fatalf("requeued job: %s %v %d", status, leaseOwner, claims)
	}
}

func TestLostLeaseCancelsTheHandlerAndRecordsNothing(t *testing.T) {
	pool := testdb.Open(t)
	fixture := newJobFixture(t, pool, "", 0)

	var cancelCause error
	handler := HandlerFunc(func(ctx context.Context, job Job) (Outcome, error) {
		// Another claim takes the job, as after an expiry the worker did not notice in time.
		if _, err := pool.Exec(ctx, "UPDATE runtime.jobs SET lease_owner = 'other-worker/new', lease_expires_at = now() + interval '1 hour' WHERE id = $1", job.ID); err != nil {
			return Outcome{}, err
		}
		select {
		case <-ctx.Done():
			cancelCause = context.Cause(ctx)
			return Outcome{}, ctx.Err()
		case <-time.After(5 * time.Second):
			return Completed(), nil
		}
	})
	logs := &bytes.Buffer{}
	worker, err := New(NewJobStore(pool), handler, Options{
		WorkerID: "worker-a", Kinds: []string{fixture.kind}, Logger: testLogger(logs),
		LeaseDuration: 3 * time.Second, RenewInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if processed, err := worker.RunOnce(context.Background()); err != nil || !processed {
		t.Fatalf("run once: %v %v", processed, err)
	}
	if !errors.Is(cancelCause, ErrLeaseLost) {
		t.Fatalf("handler was not cancelled for the lost lease: %v", cancelCause)
	}
	// The other claim keeps the job; the worker wrote nothing over it.
	if status, leaseOwner, _ := fixture.jobRow(t); status != string(JobRunning) || leaseOwner == nil || *leaseOwner != "other-worker/new" {
		t.Fatalf("job after lost lease: %s %v", status, leaseOwner)
	}
	if !strings.Contains(logs.String(), "lease renewal failed") {
		t.Fatalf("lost lease not logged: %s", logs.String())
	}
}

func TestHandlerErrorLeavesTheJobForLeaseExpiry(t *testing.T) {
	pool := testdb.Open(t)
	fixture := newJobFixture(t, pool, "", 0)
	handler := HandlerFunc(func(context.Context, Job) (Outcome, error) {
		return Completed(), errors.New("outcome unknown")
	})
	worker, err := New(NewJobStore(pool), handler, Options{
		WorkerID: "worker-a", Kinds: []string{fixture.kind}, Logger: testLogger(&bytes.Buffer{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if processed, err := worker.RunOnce(context.Background()); err != nil || !processed {
		t.Fatalf("run once: %v %v", processed, err)
	}
	// Neither completed nor failed: the worker cannot know what the handler committed.
	if status, leaseOwner, _ := fixture.jobRow(t); status != string(JobRunning) || leaseOwner == nil {
		t.Fatalf("job after handler error: %s %v", status, leaseOwner)
	}
}

func TestRunStopsWhenTheContextIsCancelled(t *testing.T) {
	queue := &emptyQueue{claimCalls: make(chan struct{}, 1000)}
	worker, err := newWorker(queue, HandlerFunc(func(context.Context, Job) (Outcome, error) { return Completed(), nil }), Options{
		WorkerID: "worker-a", Kinds: []string{"kind"}, Logger: testLogger(&bytes.Buffer{}), PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- worker.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("run returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not stop after cancellation")
	}
	if queue.claims() < 2 {
		t.Fatalf("run polled %d times, want repeated polling", queue.claims())
	}
}

// emptyQueue is a test double that never has a job.
type emptyQueue struct{ claimCalls chan struct{} }

func (queue *emptyQueue) claims() int { return len(queue.claimCalls) }

func (queue *emptyQueue) Claim(context.Context, string, []string, time.Duration) (Job, bool, error) {
	select {
	case queue.claimCalls <- struct{}{}:
	default:
	}
	return Job{}, false, nil
}
func (*emptyQueue) Renew(context.Context, Job, time.Duration) (time.Time, error) {
	return time.Time{}, ErrLeaseLost
}
func (*emptyQueue) Finish(context.Context, Job, JobStatus) error      { return ErrLeaseLost }
func (*emptyQueue) Release(context.Context, Job, time.Duration) error { return ErrLeaseLost }
