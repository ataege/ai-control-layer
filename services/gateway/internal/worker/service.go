package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// cancelGrace is how long Stop waits for a cancelled handler to return after the drain deadline.
const cancelGrace = 500 * time.Millisecond

var (
	// ErrDrainTimeout means the current step did not finish within the drain deadline and its
	// handler was cancelled. The job keeps its lease and is claimed again after the lease expires.
	ErrDrainTimeout = errors.New("worker did not finish its current step before the deadline")
	// errShuttingDown is the cancellation cause a handler sees when the drain deadline passes.
	errShuttingDown = errors.New("gateway shutting down")
)

// Service runs one worker loop in the background of the gateway process and reports whether that
// loop is running, for the readiness check (decision 5: shutdown and readiness cover the worker).
type Service struct {
	worker *Worker

	startOnce      sync.Once
	stopOnce       sync.Once
	running        atomic.Bool
	stopClaiming   context.CancelFunc
	cancelHandlers context.CancelCauseFunc
	loopDone       chan struct{}
	stopResult     error
}

// NewService wraps a worker; nothing runs until Start.
func NewService(worker *Worker) (*Service, error) {
	if worker == nil {
		return nil, ErrInvalid
	}
	return &Service{worker: worker, loopDone: make(chan struct{})}, nil
}

// Start launches the worker loop once. Later calls do nothing.
func (service *Service) Start() {
	service.startOnce.Do(func() {
		claimContext, stopClaiming := context.WithCancel(context.Background())
		handlerParent, cancelHandlers := context.WithCancelCause(context.Background())
		service.stopClaiming = stopClaiming
		service.cancelHandlers = cancelHandlers
		service.running.Store(true)
		go func() {
			defer close(service.loopDone)
			defer service.running.Store(false)
			service.worker.run(claimContext, handlerParent)
		}()
	})
}

// Ready reports whether the worker loop is running and not stopping.
func (service *Service) Ready() bool { return service.running.Load() }

// Stop stops claiming at once, lets the step in progress finish until drain has passed, then
// cancels its handler. It returns when the loop has ended, or ErrDrainTimeout when the handler
// had to be cancelled. Call it before closing the database pool. Later calls return the first
// result.
func (service *Service) Stop(drain time.Duration) error {
	service.stopOnce.Do(func() {
		// A service that never started has nothing to stop; mark Start as used so it never runs.
		started := true
		service.startOnce.Do(func() { started = false })
		if !started {
			return
		}
		service.running.Store(false)
		service.stopClaiming()
		drainTimer := time.NewTimer(drain)
		defer drainTimer.Stop()
		select {
		case <-service.loopDone:
			service.cancelHandlers(nil)
			return
		case <-drainTimer.C:
		}
		service.cancelHandlers(errShuttingDown)
		service.stopResult = ErrDrainTimeout
		graceTimer := time.NewTimer(cancelGrace)
		defer graceTimer.Stop()
		select {
		case <-service.loopDone:
		case <-graceTimer.C:
		}
	})
	return service.stopResult
}
