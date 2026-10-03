package worker

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"
)

var (
	errHandlerPanicked = errors.New("job handler panicked")
	errNoOutcome       = errors.New("job handler returned no outcome")
)

const (
	// finishTimeout bounds the write of a job outcome.
	finishTimeout = 2 * time.Second
	// DefaultLeaseDuration bounds how long a crashed worker keeps a job before it can be claimed again.
	DefaultLeaseDuration = 30 * time.Second
	// DefaultPollInterval is the wait between claim attempts when no job is available.
	DefaultPollInterval = time.Second
)

// Outcome tells the worker what to do with the job after the handler returns.
type Outcome struct {
	status JobStatus
	delay  time.Duration
}

// Completed finishes the job.
func Completed() Outcome { return Outcome{status: JobCompleted} }

// Failed finishes the job as failed. The run's own state and reason are the handler's to record.
func Failed() Outcome { return Outcome{status: JobFailed} }

// Requeue releases the lease and makes the job claimable again after delay.
func Requeue(delay time.Duration) Outcome { return Outcome{status: JobQueued, delay: delay} }

// Handler processes one claimed job. Its context is cancelled when the lease is lost, so it must
// stop before committing anything further. Returning an error leaves the job unchanged: its lease
// expires and the job is claimed again, because the worker cannot tell what the handler committed.
type Handler interface {
	Handle(ctx context.Context, job Job) (Outcome, error)
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, job Job) (Outcome, error)

// Handle calls the function.
func (function HandlerFunc) Handle(ctx context.Context, job Job) (Outcome, error) {
	return function(ctx, job)
}

// jobQueue is the part of JobStore the worker uses.
type jobQueue interface {
	Claim(ctx context.Context, workerID string, kinds []string, lease time.Duration) (Job, bool, error)
	Renew(ctx context.Context, job Job, lease time.Duration) (time.Time, error)
	Finish(ctx context.Context, job Job, status JobStatus) error
	Release(ctx context.Context, job Job, delay time.Duration) error
}

// Options configure one worker. Zero durations take the defaults; RenewInterval defaults to a
// third of the lease.
type Options struct {
	WorkerID      string
	Kinds         []string
	LeaseDuration time.Duration
	RenewInterval time.Duration
	PollInterval  time.Duration
	Logger        *slog.Logger
}

// Worker claims jobs of its kinds and runs them one at a time under a renewed lease.
type Worker struct {
	queue         jobQueue
	handler       Handler
	workerID      string
	kinds         []string
	leaseDuration time.Duration
	renewInterval time.Duration
	pollInterval  time.Duration
	logger        *slog.Logger
}

// New validates the options and returns a worker.
func New(store *JobStore, handler Handler, options Options) (*Worker, error) {
	// A nil *JobStore would become a non-nil interface value inside newWorker.
	if store == nil {
		return nil, ErrInvalid
	}
	return newWorker(store, handler, options)
}

func newWorker(queue jobQueue, handler Handler, options Options) (*Worker, error) {
	if queue == nil || handler == nil || options.Logger == nil || !validWorkerID(options.WorkerID) || len(options.Kinds) == 0 {
		return nil, ErrInvalid
	}
	for _, kind := range options.Kinds {
		if strings.TrimSpace(kind) == "" {
			return nil, ErrInvalid
		}
	}
	worker := &Worker{
		queue:         queue,
		handler:       handler,
		workerID:      options.WorkerID,
		kinds:         append([]string(nil), options.Kinds...),
		leaseDuration: options.LeaseDuration,
		renewInterval: options.RenewInterval,
		pollInterval:  options.PollInterval,
		logger:        options.Logger,
	}
	if worker.leaseDuration == 0 {
		worker.leaseDuration = DefaultLeaseDuration
	}
	if worker.renewInterval == 0 {
		worker.renewInterval = worker.leaseDuration / 3
	}
	if worker.pollInterval == 0 {
		worker.pollInterval = DefaultPollInterval
	}
	// Renewal must happen well before expiry, or a slow handler loses its job to another claim.
	if !validLease(worker.leaseDuration) || worker.renewInterval <= 0 || worker.renewInterval >= worker.leaseDuration || worker.pollInterval <= 0 {
		return nil, ErrInvalid
	}
	return worker, nil
}

// Run claims and processes jobs until ctx is cancelled. Cancelling ctx also cancels a running
// handler; Service stops claiming first and lets the current step finish. Storage errors are
// logged and retried after the poll interval; they never count as a processed job.
func (worker *Worker) Run(ctx context.Context) error {
	worker.run(ctx, ctx)
	return nil
}

// run claims while claimContext is live; handlers run under handlerParent, so a shutdown can stop
// claiming without interrupting the step in progress.
func (worker *Worker) run(claimContext, handlerParent context.Context) {
	for claimContext.Err() == nil {
		processed, err := worker.runOnce(claimContext, handlerParent)
		if err != nil && claimContext.Err() == nil {
			worker.logger.Warn("worker step failed", "worker_id", worker.workerID, "error", err.Error())
		}
		if processed && err == nil {
			continue
		}
		timer := time.NewTimer(worker.pollInterval)
		select {
		case <-claimContext.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

// RunOnce claims at most one job and processes it. It reports whether a job was claimed.
func (worker *Worker) RunOnce(ctx context.Context) (bool, error) {
	return worker.runOnce(ctx, ctx)
}

func (worker *Worker) runOnce(claimContext, handlerParent context.Context) (bool, error) {
	job, claimed, err := worker.queue.Claim(claimContext, worker.workerID, worker.kinds, worker.leaseDuration)
	if err != nil || !claimed {
		return false, err
	}
	jobLogger := worker.logger.With("worker_id", worker.workerID, "job_id", job.ID, "run_id", job.RunID, "kind", job.Kind)

	handlerContext, cancelHandler := context.WithCancelCause(handlerParent)
	renewalDone := make(chan struct{})
	go worker.keepLease(handlerContext, cancelHandler, job, jobLogger, renewalDone)

	outcome, handlerErr := worker.handle(handlerContext, job)
	cancelHandler(nil)
	<-renewalDone

	if handlerErr != nil {
		// Leave the job as it is: the lease expires and the job is claimed again.
		jobLogger.Warn("job handler failed; the lease will expire", "error", handlerErr.Error())
		return true, nil
	}
	// The outcome is written even during shutdown: a short write is better than a redundant
	// replay. The bound keeps it inside the gateway's shutdown budget.
	finishContext, cancelFinish := context.WithTimeout(context.WithoutCancel(handlerParent), finishTimeout)
	defer cancelFinish()
	var finishErr error
	if outcome.status == JobQueued {
		finishErr = worker.queue.Release(finishContext, job, outcome.delay)
	} else {
		finishErr = worker.queue.Finish(finishContext, job, outcome.status)
	}
	if finishErr != nil {
		jobLogger.Warn("job outcome not recorded", "error", finishErr.Error())
		return true, finishErr
	}
	return true, nil
}

// handle runs the handler and turns a panic into a handler error, so one bad job cannot stop
// the gateway process; the job is then left for lease expiry like any other handler error.
func (worker *Worker) handle(ctx context.Context, job Job) (outcome Outcome, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			worker.logger.Error("job handler panicked", "worker_id", worker.workerID, "job_id", job.ID, "stack", string(debug.Stack()))
			outcome, err = Outcome{}, errHandlerPanicked
		}
	}()
	if outcome, err = worker.handler.Handle(ctx, job); err == nil && outcome.status == "" {
		// The zero Outcome is not a decision; treat it like an error rather than guessing.
		err = errNoOutcome
	}
	return outcome, err
}

// keepLease renews the lease until the handler returns. Any renewal failure cancels the handler:
// without a confirmed lease another worker may already own the job.
func (worker *Worker) keepLease(ctx context.Context, cancelHandler context.CancelCauseFunc, job Job, jobLogger *slog.Logger, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(worker.renewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := worker.queue.Renew(ctx, job, worker.leaseDuration); err != nil {
				if ctx.Err() != nil {
					return
				}
				if !errors.Is(err, ErrLeaseLost) {
					err = errors.Join(ErrLeaseLost, err)
				}
				jobLogger.Warn("job lease renewal failed; stopping the handler", "error", err.Error())
				cancelHandler(err)
				return
			}
		}
	}
}
