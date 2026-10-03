package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/testdb"
)

func TestStoreFailsClosedWithoutDatabaseOrValidInput(t *testing.T) {
	ctx := context.Background()
	unavailable := NewJobStore(nil)
	if _, _, err := unavailable.Claim(ctx, "worker-a", []string{"kind"}, time.Second); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("claim without pool: %v", err)
	}
	if err := unavailable.Finish(ctx, Job{ID: "job", LeaseToken: "token"}, JobCompleted); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("finish without pool: %v", err)
	}

	store := &JobStore{pool: &pgxpool.Pool{}}
	for name, claim := range map[string]func() error{
		"empty worker id":  func() error { _, _, err := store.Claim(ctx, "", []string{"kind"}, time.Second); return err },
		"unsafe worker id": func() error { _, _, err := store.Claim(ctx, "worker/a", []string{"kind"}, time.Second); return err },
		"no kinds":         func() error { _, _, err := store.Claim(ctx, "worker-a", nil, time.Second); return err },
		"blank kind":       func() error { _, _, err := store.Claim(ctx, "worker-a", []string{" "}, time.Second); return err },
		"lease too short": func() error {
			_, _, err := store.Claim(ctx, "worker-a", []string{"kind"}, time.Millisecond)
			return err
		},
		"lease too long": func() error { _, _, err := store.Claim(ctx, "worker-a", []string{"kind"}, 2*time.Hour); return err },
	} {
		if err := claim(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	job := Job{ID: "job", LeaseToken: "token"}
	if err := store.Finish(ctx, job, JobQueued); !errors.Is(err, ErrInvalid) {
		t.Errorf("finish to a non-terminal status: %v", err)
	}
	if err := store.Finish(ctx, job, JobRunning); !errors.Is(err, ErrInvalid) {
		t.Errorf("finish to running: %v", err)
	}
	if err := store.Release(ctx, job, -time.Second); !errors.Is(err, ErrInvalid) {
		t.Errorf("negative release delay: %v", err)
	}
	if _, err := store.Renew(ctx, Job{ID: "job"}, time.Second); !errors.Is(err, ErrInvalid) {
		t.Errorf("renew without token: %v", err)
	}
}

// jobFixture is one synthetic passport, run and job; the kind is unique per test so concurrent
// test packages and other tests never claim each other's jobs.
type jobFixture struct {
	pool  *pgxpool.Pool
	jobID string
	runID string
	kind  string
}

// newJobFixture inserts a passport, run and job. The job starts queued unless the caller passes
// a lease owner and expiry offset, which make it a running job with that lease.
func newJobFixture(t *testing.T, pool *pgxpool.Pool, leaseOwner string, leaseOffset time.Duration) jobFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fixture := jobFixture{pool: pool, jobID: testdb.ID(t), runID: testdb.ID(t), kind: "worker-test-" + testdb.ID(t)}
	organizationID, passportID, actorID := testdb.ID(t), testdb.ID(t), testdb.ID(t)
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal("begin fixture transaction")
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO runtime.passports(id, organization_id, actor_id, task_version, admission_catalog_revision_id, scope, limits, expires_at)
		  VALUES ($1, $2, $3, 'worker_test_v1', 1, '{}', '{}', now() + interval '1 hour')`, []any{passportID, organizationID, actorID}},
		{`INSERT INTO runtime.runs(id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
			[]any{fixture.runID, organizationID, passportID}},
	}
	if leaseOwner == "" {
		statements = append(statements, struct {
			sql  string
			args []any
		}{`INSERT INTO runtime.jobs(id, organization_id, run_id, kind, status) VALUES ($1, $2, $3, $4, 'queued')`,
			[]any{fixture.jobID, organizationID, fixture.runID, fixture.kind}})
	} else {
		statements = append(statements, struct {
			sql  string
			args []any
		}{`INSERT INTO runtime.jobs(id, organization_id, run_id, kind, status, lease_owner, lease_expires_at)
		   VALUES ($1, $2, $3, $4, 'running', $5, now() + ($6 * interval '1 millisecond'))`,
			[]any{fixture.jobID, organizationID, fixture.runID, fixture.kind, leaseOwner, leaseOffset.Milliseconds()}})
	}
	for _, statement := range statements {
		if _, err = transaction.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("insert job fixture: %v", err)
		}
	}
	if err = transaction.Commit(ctx); err != nil {
		t.Fatal("commit job fixture")
	}
	t.Cleanup(func() { fixture.remove(t, passportID) })
	return fixture
}

// remove deletes the fixture. Passports reject DELETE by trigger, so the passport row is removed
// with triggers disabled for this transaction, which needs a superuser test database; otherwise
// the passport row (a unique synthetic id) stays behind and the test only logs it.
func (fixture jobFixture) remove(t *testing.T, passportID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := fixture.pool.Exec(ctx, "DELETE FROM runtime.jobs WHERE id = $1", fixture.jobID); err != nil {
		t.Error("could not clean the job fixture")
		return
	}
	if _, err := fixture.pool.Exec(ctx, "DELETE FROM runtime.runs WHERE id = $1", fixture.runID); err != nil {
		t.Error("could not clean the run fixture")
		return
	}
	transaction, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Error("could not clean the passport fixture")
		return
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err = transaction.Exec(ctx, "SET LOCAL session_replication_role = replica"); err == nil {
		_, err = transaction.Exec(ctx, "DELETE FROM runtime.passports WHERE id = $1", passportID)
	}
	if err == nil {
		err = transaction.Commit(ctx)
	}
	if err != nil {
		t.Logf("passport fixture %s left in place (removing it needs a superuser)", passportID)
	}
}

// jobRow reads the stored lease fields of the fixture job.
func (fixture jobFixture) jobRow(t *testing.T) (status string, leaseOwner *string, claimCount int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := fixture.pool.QueryRow(ctx, "SELECT status, lease_owner, attempt_count FROM runtime.jobs WHERE id = $1", fixture.jobID).
		Scan(&status, &leaseOwner, &claimCount)
	if err != nil {
		t.Fatal("read job fixture")
	}
	return status, leaseOwner, claimCount
}

func TestConcurrentClaimsYieldOneOwner(t *testing.T) {
	pool := testdb.Open(t)
	fixture := newJobFixture(t, pool, "", 0)
	store := NewJobStore(pool)

	const claimers = 8
	var waitGroup sync.WaitGroup
	results := make(chan Job, claimers)
	errorsSeen := make(chan error, claimers)
	start := make(chan struct{})
	for index := range claimers {
		waitGroup.Add(1)
		go func(workerNumber int) {
			defer waitGroup.Done()
			<-start
			job, claimed, err := store.Claim(context.Background(), "racer-"+string(rune('a'+workerNumber)), []string{fixture.kind}, 10*time.Second)
			if err != nil {
				errorsSeen <- err
				return
			}
			if claimed {
				results <- job
			}
		}(index)
	}
	close(start)
	waitGroup.Wait()
	close(results)
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatalf("claim failed: %v", err)
	}
	var owners []Job
	for job := range results {
		owners = append(owners, job)
	}
	if len(owners) != 1 {
		t.Fatalf("%d claimers won the same job, want exactly 1", len(owners))
	}
	status, leaseOwner, claimCount := fixture.jobRow(t)
	if status != string(JobRunning) || leaseOwner == nil || *leaseOwner != owners[0].LeaseToken || claimCount != 1 {
		t.Fatalf("stored job: status %s, owner %v, claims %d", status, leaseOwner, claimCount)
	}
}

func TestLiveLeaseCannotBeClaimed(t *testing.T) {
	pool := testdb.Open(t)
	fixture := newJobFixture(t, pool, "", 0)
	store := NewJobStore(pool)
	ctx := context.Background()

	first, claimed, err := store.Claim(ctx, "worker-a", []string{fixture.kind}, 10*time.Second)
	if err != nil || !claimed || first.ID != fixture.jobID || first.RunID != fixture.runID {
		t.Fatalf("first claim: %+v %v %v", first, claimed, err)
	}
	if _, claimed, err = store.Claim(ctx, "worker-b", []string{fixture.kind}, 10*time.Second); err != nil || claimed {
		t.Fatalf("a job under a live lease was claimed again: %v %v", claimed, err)
	}
	// A renewal by the holder succeeds; a finished job is never claimed again.
	if _, err = store.Renew(ctx, first, 10*time.Second); err != nil {
		t.Fatalf("renew live lease: %v", err)
	}
	if err = store.Finish(ctx, first, JobCompleted); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if _, claimed, err = store.Claim(ctx, "worker-b", []string{fixture.kind}, 10*time.Second); err != nil || claimed {
		t.Fatalf("a completed job was claimed: %v %v", claimed, err)
	}
	status, leaseOwner, _ := fixture.jobRow(t)
	if status != string(JobCompleted) || leaseOwner != nil {
		t.Fatalf("completed job kept its lease: %s %v", status, leaseOwner)
	}
}

func TestExpiredLeaseIsClaimedAgainAndFencesTheOldClaim(t *testing.T) {
	pool := testdb.Open(t)
	// A job left by a crashed worker: running, its lease already past expiry.
	fixture := newJobFixture(t, pool, "crashed-worker/old", -time.Second)
	store := NewJobStore(pool)
	ctx := context.Background()

	staleClaim := Job{ID: fixture.jobID, LeaseToken: "crashed-worker/old"}
	if _, err := store.Renew(ctx, staleClaim, 10*time.Second); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("renewal of an expired lease: %v", err)
	}
	first, claimed, err := store.Claim(ctx, "worker-a", []string{fixture.kind}, 10*time.Second)
	if err != nil || !claimed {
		t.Fatalf("claim of an expired lease: %v %v", claimed, err)
	}

	// The same worker loses its own lease to expiry and claims the job again: the first claim's
	// token must no longer renew, release or finish anything.
	if _, err = pool.Exec(ctx, "UPDATE runtime.jobs SET lease_expires_at = now() - interval '1 second' WHERE id = $1", fixture.jobID); err != nil {
		t.Fatal("expire lease")
	}
	second, claimed, err := store.Claim(ctx, "worker-a", []string{fixture.kind}, 10*time.Second)
	if err != nil || !claimed || second.LeaseToken == first.LeaseToken || second.ClaimCount != first.ClaimCount+1 {
		t.Fatalf("second claim: %+v %v %v", second, claimed, err)
	}
	if _, err = store.Renew(ctx, first, 10*time.Second); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("old claim renewed: %v", err)
	}
	if err = store.Finish(ctx, first, JobCompleted); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("old claim finished the job: %v", err)
	}
	if err = store.Release(ctx, first, 0); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("old claim released the job: %v", err)
	}
	if status, leaseOwner, _ := fixture.jobRow(t); status != string(JobRunning) || leaseOwner == nil || *leaseOwner != second.LeaseToken {
		t.Fatalf("old claim changed the job: %s %v", status, leaseOwner)
	}

	// A released job becomes claimable once its delay has passed on the database clock.
	if err = store.Release(ctx, second, time.Hour); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, claimed, err = store.Claim(ctx, "worker-b", []string{fixture.kind}, 10*time.Second); err != nil || claimed {
		t.Fatalf("a delayed job was claimed early: %v %v", claimed, err)
	}
}

func TestClaimedJobSurvivesANewPool(t *testing.T) {
	// The fixture lives on its own pool so cleanup still works after the claiming pool closes.
	fixture := newJobFixture(t, testdb.Open(t), "", 0)
	firstPool := testdb.Open(t)
	claimed, ok, err := NewJobStore(firstPool).Claim(context.Background(), "worker-a", []string{fixture.kind}, 10*time.Second)
	if err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	firstPool.Close()

	secondPool := testdb.Open(t)
	fixture.pool = secondPool
	status, leaseOwner, _ := fixture.jobRow(t)
	if status != string(JobRunning) || leaseOwner == nil || *leaseOwner != claimed.LeaseToken {
		t.Fatalf("claim not durable: %s %v", status, leaseOwner)
	}
	secondStore := NewJobStore(secondPool)
	if _, ok, err = secondStore.Claim(context.Background(), "worker-b", []string{fixture.kind}, 10*time.Second); err != nil || ok {
		t.Fatalf("the live lease did not survive the pool: %v %v", ok, err)
	}
	if err = secondStore.Finish(context.Background(), claimed, JobCompleted); err != nil {
		t.Fatalf("the claim could not finish through a new pool: %v", err)
	}
}
