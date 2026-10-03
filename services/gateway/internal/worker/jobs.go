// Package worker claims durable runtime jobs with a lease and processes them one at a time
// (decision 5: PostgreSQL jobs with leases, no broker, one worker process).
package worker

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobStatus is the Go-internal state of a runtime.jobs row. It is not a wire contract: the run
// state (X-11) is the contract the interface reads.
type JobStatus string

const (
	// JobQueued is claimable once available_at has passed.
	JobQueued JobStatus = "queued"
	// JobRunning is held under a lease; it becomes claimable again only after the lease expires.
	JobRunning JobStatus = "running"
	// JobCompleted and JobFailed are terminal and never claimed again.
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
)

var (
	// ErrLeaseLost means the caller no longer holds the lease: it expired or another claim took it.
	// The caller must stop touching the job.
	ErrLeaseLost = errors.New("job lease lost")
	// ErrInvalid rejects an unusable argument before any database work.
	ErrInvalid = errors.New("invalid job operation")
	// ErrUnavailable hides driver errors, which can name hosts and users.
	ErrUnavailable = errors.New("job storage unavailable")
)

// Job is one claimed runtime job. LeaseToken fences every later write to it.
type Job struct {
	ID             string
	OrganizationID string
	RunID          string
	Kind           string
	// ActionID is the stored action a continuation resumes; empty when the job has none.
	ActionID string
	// ClaimCount counts claims of this job (runtime.jobs.attempt_count). It is not a dispatch
	// attempt: model_calls and execution_attempts are the dispatch records (GO-02).
	ClaimCount     int
	LeaseToken     string
	LeaseExpiresAt time.Time
}

// JobStore reads and writes runtime.jobs. Each method is one statement, so no transaction or
// row lock outlives the call: nothing is held across a model or tool request.
type JobStore struct{ pool *pgxpool.Pool }

// NewJobStore returns a store over the shared gateway pool.
func NewJobStore(pool *pgxpool.Pool) *JobStore { return &JobStore{pool: pool} }

// Claim takes the oldest available job of one of the given kinds, or reports false when none is
// available. A queued job and a running job whose lease has expired are both claimable; SKIP
// LOCKED lets concurrent claimers pass over a row another claimer is taking. Expiry is always
// compared with the database clock, never with this process's clock.
func (store *JobStore) Claim(ctx context.Context, workerID string, kinds []string, lease time.Duration) (Job, bool, error) {
	if store == nil || store.pool == nil {
		return Job{}, false, ErrUnavailable
	}
	if !validWorkerID(workerID) || len(kinds) == 0 || !validLease(lease) {
		return Job{}, false, ErrInvalid
	}
	for _, kind := range kinds {
		if strings.TrimSpace(kind) == "" {
			return Job{}, false, ErrInvalid
		}
	}
	leaseToken, err := newLeaseToken(workerID)
	if err != nil {
		return Job{}, false, ErrUnavailable
	}
	var job Job
	err = store.pool.QueryRow(ctx, `
		UPDATE runtime.jobs AS job
		SET status = 'running',
		    lease_owner = $1,
		    lease_expires_at = now() + ($2 * interval '1 millisecond'),
		    attempt_count = job.attempt_count + 1,
		    updated_at = now()
		WHERE job.id = (
			SELECT candidate.id FROM runtime.jobs AS candidate
			WHERE candidate.kind = ANY($3)
			  AND candidate.available_at <= now()
			  AND (candidate.status = 'queued'
			       OR (candidate.status = 'running' AND candidate.lease_expires_at <= now()))
			ORDER BY candidate.available_at, candidate.id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING job.id::text, job.organization_id::text, job.run_id::text, job.kind,
		          COALESCE(job.action_id::text, ''), job.attempt_count, job.lease_owner, job.lease_expires_at`,
		leaseToken, lease.Milliseconds(), kinds,
	).Scan(&job.ID, &job.OrganizationID, &job.RunID, &job.Kind, &job.ActionID, &job.ClaimCount, &job.LeaseToken, &job.LeaseExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, ErrUnavailable
	}
	return job, true, nil
}

// Renew extends a live lease held by this job's token and returns the new expiry.
func (store *JobStore) Renew(ctx context.Context, job Job, lease time.Duration) (time.Time, error) {
	if store == nil || store.pool == nil {
		return time.Time{}, ErrUnavailable
	}
	if job.ID == "" || job.LeaseToken == "" || !validLease(lease) {
		return time.Time{}, ErrInvalid
	}
	var expiresAt time.Time
	err := store.pool.QueryRow(ctx, `
		UPDATE runtime.jobs
		SET lease_expires_at = now() + ($3 * interval '1 millisecond'), updated_at = now()
		WHERE id = $1 AND lease_owner = $2 AND status = 'running' AND lease_expires_at > now()
		RETURNING lease_expires_at`,
		job.ID, job.LeaseToken, lease.Milliseconds(),
	).Scan(&expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrLeaseLost
	}
	if err != nil {
		return time.Time{}, ErrUnavailable
	}
	return expiresAt, nil
}

// Finish moves a leased job to a terminal status and clears its lease.
func (store *JobStore) Finish(ctx context.Context, job Job, status JobStatus) error {
	if status != JobCompleted && status != JobFailed {
		return ErrInvalid
	}
	return store.endLease(ctx, job, status, 0)
}

// Release clears the lease and makes the job claimable again after delay, measured on the
// database clock. GO-40 adds the review-wait status that is released without becoming claimable.
func (store *JobStore) Release(ctx context.Context, job Job, delay time.Duration) error {
	if delay < 0 {
		return ErrInvalid
	}
	return store.endLease(ctx, job, JobQueued, delay)
}

func (store *JobStore) endLease(ctx context.Context, job Job, status JobStatus, delay time.Duration) error {
	if store == nil || store.pool == nil {
		return ErrUnavailable
	}
	if job.ID == "" || job.LeaseToken == "" {
		return ErrInvalid
	}
	tag, err := store.pool.Exec(ctx, `
		UPDATE runtime.jobs
		SET status = $3,
		    lease_owner = NULL,
		    lease_expires_at = NULL,
		    available_at = CASE WHEN $3 = 'queued' THEN now() + ($4 * interval '1 millisecond') ELSE available_at END,
		    updated_at = now()
		WHERE id = $1 AND lease_owner = $2 AND status = 'running' AND lease_expires_at > now()`,
		job.ID, job.LeaseToken, string(status), delay.Milliseconds(),
	)
	if err != nil {
		return ErrUnavailable
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return nil
}

// validLease keeps leases between 100 ms and one hour; they are stored in whole milliseconds.
func validLease(lease time.Duration) bool {
	return lease >= 100*time.Millisecond && lease <= time.Hour
}

// validWorkerID bounds the worker part of the lease token before it reaches the database.
func validWorkerID(workerID string) bool {
	if workerID == "" || len(workerID) > 64 {
		return false
	}
	for _, character := range workerID {
		isLetter := (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
		isDigit := character >= '0' && character <= '9'
		if !isLetter && !isDigit && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

// newLeaseToken makes a fresh token per claim, so a worker whose own lease expired and was
// claimed again (even by the same worker) cannot renew or finish through the old claim.
func newLeaseToken(workerID string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%x", workerID, random), nil
}
