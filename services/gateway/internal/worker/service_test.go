package worker

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"starter/services/gateway/internal/testdb"
)

// scriptedQueue is a test double that hands out one job and records what the worker wrote.
type scriptedQueue struct {
	mutex    sync.Mutex
	handed   bool
	finished []JobStatus
	claims   int
}

func (queue *scriptedQueue) Claim(ctx context.Context, _ string, _ []string, lease time.Duration) (Job, bool, error) {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()
	queue.claims++
	if queue.handed {
		return Job{}, false, nil
	}
	queue.handed = true
	return Job{ID: "job-1", RunID: "run-1", Kind: "kind", LeaseToken: "worker-a/1", LeaseExpiresAt: time.Now().Add(lease)}, true, nil
}
func (queue *scriptedQueue) Renew(context.Context, Job, time.Duration) (time.Time, error) {
	return time.Now().Add(time.Minute), nil
}
func (queue *scriptedQueue) Finish(_ context.Context, _ Job, status JobStatus) error {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()
	queue.finished = append(queue.finished, status)
	return nil
}
func (queue *scriptedQueue) Release(context.Context, Job, time.Duration) error {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()
	queue.finished = append(queue.finished, JobQueued)
	return nil
}
func (queue *scriptedQueue) snapshot() (claims int, finished []JobStatus) {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()
	return queue.claims, append([]JobStatus(nil), queue.finished...)
}

func startedService(t *testing.T, queue jobQueue, handler Handler) *Service {
	t.Helper()
	worker, err := newWorker(queue, handler, Options{
		WorkerID: "worker-a", Kinds: []string{"kind"}, Logger: testLogger(&bytes.Buffer{}), PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(worker)
	if err != nil {
		t.Fatal(err)
	}
	service.Start()
	return service
}

func TestStopLetsTheCurrentStepFinishAndStopsClaiming(t *testing.T) {
	queue := &scriptedQueue{}
	stepStarted := make(chan struct{})
	var handlerCancelled bool
	service := startedService(t, queue, HandlerFunc(func(ctx context.Context, job Job) (Outcome, error) {
		close(stepStarted)
		// A step that finishes shortly after the stop signal: it must not be interrupted.
		select {
		case <-time.After(150 * time.Millisecond):
		case <-ctx.Done():
			handlerCancelled = true
		}
		return Completed(), nil
	}))
	<-stepStarted
	if !service.Ready() {
		t.Fatal("a running worker reported not ready")
	}
	if err := service.Stop(2 * time.Second); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if service.Ready() {
		t.Fatal("a stopped worker reported ready")
	}
	claimsAtStop, finished := queue.snapshot()
	if handlerCancelled {
		t.Fatal("the step in progress was cancelled although it finished within the drain deadline")
	}
	if len(finished) != 1 || finished[0] != JobCompleted {
		t.Fatalf("the finished step's outcome was not recorded: %v", finished)
	}
	time.Sleep(50 * time.Millisecond)
	if claimsAfter, _ := queue.snapshot(); claimsAfter != claimsAtStop {
		t.Fatalf("the worker kept claiming after Stop: %d then %d", claimsAtStop, claimsAfter)
	}
}

func TestStopCancelsAStepThatOverrunsTheDeadline(t *testing.T) {
	queue := &scriptedQueue{}
	stepStarted := make(chan struct{})
	var cancelCause error
	service := startedService(t, queue, HandlerFunc(func(ctx context.Context, job Job) (Outcome, error) {
		close(stepStarted)
		<-ctx.Done()
		cancelCause = context.Cause(ctx)
		return Outcome{}, ctx.Err()
	}))
	<-stepStarted
	started := time.Now()
	err := service.Stop(100 * time.Millisecond)
	if !errors.Is(err, ErrDrainTimeout) {
		t.Fatalf("stop: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond+cancelGrace {
		t.Fatalf("stop took %v, beyond the drain deadline and grace", elapsed)
	}
	if !errors.Is(cancelCause, errShuttingDown) {
		t.Fatalf("handler cancel cause: %v", cancelCause)
	}
	// An interrupted step records no outcome: the job is left for lease expiry.
	if _, finished := queue.snapshot(); len(finished) != 0 {
		t.Fatalf("an interrupted step recorded %v", finished)
	}
	if again := service.Stop(time.Second); !errors.Is(again, ErrDrainTimeout) {
		t.Fatalf("second stop: %v", again)
	}
}

func TestStopBeforeStartAndPanicsAreSafe(t *testing.T) {
	worker, err := newWorker(&scriptedQueue{}, HandlerFunc(func(context.Context, Job) (Outcome, error) { panic("boom") }), Options{
		WorkerID: "worker-a", Kinds: []string{"kind"}, Logger: testLogger(&bytes.Buffer{}), PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	unstarted, _ := NewService(worker)
	if err = unstarted.Stop(time.Second); err != nil || unstarted.Ready() {
		t.Fatalf("stop before start: %v ready=%v", err, unstarted.Ready())
	}
	unstarted.Start()
	if unstarted.Ready() {
		t.Fatal("a stopped service started again")
	}
	if _, err = NewService(nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil worker: %v", err)
	}

	// A panicking handler is contained: the loop keeps running and records no outcome.
	queue := &scriptedQueue{}
	logs := &bytes.Buffer{}
	panicking, _ := newWorker(queue, HandlerFunc(func(context.Context, Job) (Outcome, error) { panic("boom") }), Options{
		WorkerID: "worker-a", Kinds: []string{"kind"}, Logger: testLogger(logs), PollInterval: 10 * time.Millisecond,
	})
	if processed, err := panicking.RunOnce(context.Background()); err != nil || !processed {
		t.Fatalf("run once with a panicking handler: %v %v", processed, err)
	}
	if _, finished := queue.snapshot(); len(finished) != 0 {
		t.Fatalf("a panicking step recorded %v", finished)
	}
	if !bytes.Contains(logs.Bytes(), []byte("job handler panicked")) {
		t.Fatal("the panic was not logged")
	}

	// The zero Outcome is not a decision either.
	zeroQueue := &scriptedQueue{}
	zero, _ := newWorker(zeroQueue, HandlerFunc(func(context.Context, Job) (Outcome, error) { return Outcome{}, nil }), Options{
		WorkerID: "worker-a", Kinds: []string{"kind"}, Logger: testLogger(&bytes.Buffer{}),
	})
	if _, err := zero.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, finished := zeroQueue.snapshot(); len(finished) != 0 {
		t.Fatalf("a zero outcome recorded %v", finished)
	}
}

func TestInterruptedJobIsClaimedAgainAfterItsLeaseExpires(t *testing.T) {
	pool := testdb.Open(t)
	fixture := newJobFixture(t, pool, "", 0)
	store := NewJobStore(pool)
	stepStarted := make(chan struct{})
	worker, err := New(store, HandlerFunc(func(ctx context.Context, job Job) (Outcome, error) {
		close(stepStarted)
		<-ctx.Done()
		return Outcome{}, ctx.Err()
	}), Options{
		WorkerID: "worker-a", Kinds: []string{fixture.kind}, Logger: testLogger(&bytes.Buffer{}),
		LeaseDuration: 400 * time.Millisecond, RenewInterval: 100 * time.Millisecond, PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	service, _ := NewService(worker)
	service.Start()
	<-stepStarted
	if err = service.Stop(50 * time.Millisecond); !errors.Is(err, ErrDrainTimeout) {
		t.Fatalf("stop: %v", err)
	}
	// The interrupted job keeps its lease: nothing it committed is lost or overwritten.
	if status, leaseOwner, _ := fixture.jobRow(t); status != string(JobRunning) || leaseOwner == nil {
		t.Fatalf("interrupted job: %s %v", status, leaseOwner)
	}
	if _, claimed, err := store.Claim(context.Background(), "worker-b", []string{fixture.kind}, time.Second); err != nil || claimed {
		t.Fatalf("claimed under the old live lease: %v %v", claimed, err)
	}
	time.Sleep(500 * time.Millisecond)
	recovered, claimed, err := store.Claim(context.Background(), "worker-b", []string{fixture.kind}, time.Second)
	if err != nil || !claimed || recovered.ClaimCount != 2 {
		t.Fatalf("not claimable after expiry: %+v %v %v", recovered, claimed, err)
	}
}
