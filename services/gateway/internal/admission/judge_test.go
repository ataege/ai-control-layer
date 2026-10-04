package admission

import (
	"context"
	"errors"
	"testing"
)

// countWith counts rows of a query with arguments inside the fixture's transaction.
func countWith(t *testing.T, fixture *admissionFixture, sql string, arguments ...any) int {
	t.Helper()
	var count int
	if err := fixture.outer.QueryRow(context.Background(), sql, arguments...).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	return count
}

// A judge run (the judge console) gets the same passport, run, ledger and run.queued event as an
// agent run, but no job, so no worker ever claims it and its agent never calls the model.
func TestPostgresJudgeAdmissionEnqueuesNoAgentJob(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	judge, err := fixture.admitter.AdmitJudge(ctx, fixture.operator, fixture.request())
	if err != nil {
		t.Fatalf("AdmitJudge: %v", err)
	}
	agent, err := fixture.admitter.Admit(ctx, fixture.operator, fixture.request())
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	for name, check := range map[string]struct {
		runID     string
		wantJobs  int
		wantJudge int
	}{
		"judge run": {judge.RunID, 0, 1},
		// Normal admission still enqueues the agent's first job, unmarked.
		"agent run": {agent.RunID, 1, 0},
	} {
		if count := countWith(t, fixture, `SELECT count(*) FROM runtime.runs WHERE id = $1 AND status = 'queued'`, check.runID); count != 1 {
			t.Errorf("%s: queued runs = %d, want 1", name, count)
		}
		if count := countWith(t, fixture, `SELECT count(*) FROM runtime.passports p JOIN runtime.runs r ON r.passport_id = p.id WHERE r.id = $1`, check.runID); count != 1 {
			t.Errorf("%s: passports = %d, want 1", name, count)
		}
		if count := countWith(t, fixture, `SELECT count(*) FROM runtime.model_token_budgets WHERE run_id = $1`, check.runID); count != 1 {
			t.Errorf("%s: ledgers = %d, want 1 (evaluations meter security calls)", name, count)
		}
		if count := countWith(t, fixture, `SELECT count(*) FROM runtime.jobs WHERE run_id = $1`, check.runID); count != check.wantJobs {
			t.Errorf("%s: jobs = %d, want %d", name, count, check.wantJobs)
		}
		if count := countWith(t, fixture, `SELECT count(*) FROM runtime.audit_events WHERE run_id = $1 AND event_type = 'run.queued'
			AND masked_summary->>'inputSource' = 'judge'`, check.runID); count != check.wantJudge {
			t.Errorf("%s: judge-marked run.queued events = %d, want %d", name, count, check.wantJudge)
		}
	}
}

// A judge run is refused exactly like an agent run: same validation, no passport.
func TestPostgresJudgeAdmissionRejectsWhatAdmitRejects(t *testing.T) {
	fixture := newFixture(t)
	request := fixture.request()
	request.InvoiceIDs = append(request.InvoiceIDs, fixture.foreignInvoice)
	_, err := fixture.admitter.AdmitJudge(context.Background(), fixture.operator, request)
	var rejection *Rejection
	if !errors.As(err, &rejection) {
		t.Fatalf("got %v, want a rejection", err)
	}
	fixture.assertNothingAdmitted(t)
}
