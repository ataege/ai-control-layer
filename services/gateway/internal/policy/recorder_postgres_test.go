package policy

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/testdb"
)

// seedRun inserts a passport and its run for an isolated organization. Passports are immutable
// (no UPDATE or DELETE), so the rows stay in the test database; their organization is random.
func seedRun(t *testing.T, pool *pgxpool.Pool) RunIdentity {
	t.Helper()
	ctx := context.Background()
	run := RunIdentity{OrganizationID: testdb.ID(t), RunID: testdb.ID(t)}
	passportID := testdb.ID(t)
	if _, err := pool.Exec(ctx,
		`INSERT INTO runtime.passports (id, organization_id, actor_id, task_version, admission_catalog_revision_id, scope, limits, expires_at)
		 VALUES ($1, $2, $3, 'reconcile_atlas_v1', 1, '{}', '{}', now() + interval '15 minutes')`,
		passportID, run.OrganizationID, testdb.ID(t)); err != nil {
		t.Fatalf("seed passport: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
		run.RunID, run.OrganizationID, passportID); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	return run
}

func storedReadAction(t *testing.T, run RunIdentity, stepNumber int) StoredAction {
	t.Helper()
	arguments, err := DecodeArguments(ToolReadInvoice, []byte(`{"invoice_id":"invoice_A01"}`))
	if err != nil {
		t.Fatal(err)
	}
	canonicalArguments, _ := CanonicalArguments(arguments)
	action := StoredAction{
		OrganizationID: run.OrganizationID, RunID: run.RunID, ActionID: testdb.ID(t), StepNumber: stepNumber,
		IdempotencyKey: testdb.ID(t), Tool: ToolReadInvoice, CanonicalArguments: canonicalArguments,
		CanonicalizationVersion: CanonicalizationVersion, EvaluatedRevisionID: 1,
	}
	action.ActionDigest, err = CanonicalAction{
		ActionID: action.ActionID, RunID: run.RunID, Arguments: arguments,
		PassportID: testdb.ID(t), PolicyRevisionID: 1,
	}.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return action
}

func TestRecorderStoresTheActionBeforeItsDecision(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	recorder := NewPostgresRecorder(pool)
	run := seedRun(t, pool)
	action := storedReadAction(t, run, 1)

	if err := recorder.StoreAction(ctx, action); err != nil {
		t.Fatalf("store action: %v", err)
	}
	var status string
	var storedArguments string
	if err := pool.QueryRow(ctx, `SELECT status, canonical_arguments::text FROM runtime.actions WHERE id = $1`, action.ActionID).
		Scan(&status, &storedArguments); err != nil || status != actionStatusProposed {
		t.Fatalf("stored action status = %q, err %v; want proposed before any decision", status, err)
	}

	decision := Decision{Outcome: OutcomeAllow, ActionID: action.ActionID, ActionStored: true, ActionDigest: action.ActionDigest, EvaluatedRevisionID: 1}
	if err := recorder.RecordDecision(ctx, run, decision); err != nil {
		t.Fatalf("record decision: %v", err)
	}
	var eventDecision, summary string
	var actionCreated, eventOccurred time.Time
	if err := pool.QueryRow(ctx,
		`SELECT a.status, a.created_at, e.decision, e.masked_summary::text, e.occurred_at
		   FROM runtime.actions a JOIN runtime.audit_events e ON e.action_id = a.id
		  WHERE a.id = $1`, action.ActionID).Scan(&status, &actionCreated, &eventDecision, &summary, &eventOccurred); err != nil {
		t.Fatalf("read decision: %v", err)
	}
	if status != actionStatusAllowed || eventDecision != string(OutcomeAllow) {
		t.Fatalf("status %q, event decision %q; want allowed/allow", status, eventDecision)
	}
	if eventOccurred.Before(actionCreated) {
		t.Fatal("the decision event is older than its action")
	}
	if summary != `{"outcome": "allow", "action_stored": true}` {
		t.Fatalf("masked summary = %s; want references only", summary)
	}

	// A second decision for the same action is refused: the action is decided exactly once.
	if err := recorder.RecordDecision(ctx, run, decision); err == nil {
		t.Fatal("a second decision for the same action was accepted")
	}
}

func TestRecorderAcceptsTheSameActionTwiceButNotAConflict(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	recorder := NewPostgresRecorder(pool)
	run := seedRun(t, pool)
	action := storedReadAction(t, run, 1)
	if err := recorder.StoreAction(ctx, action); err != nil {
		t.Fatal(err)
	}
	if err := recorder.StoreAction(ctx, action); err != nil {
		t.Fatalf("storing the identical action again: %v", err)
	}
	changed := action
	changed.ActionDigest[0] ^= 0xff
	if err := recorder.StoreAction(ctx, changed); err == nil {
		t.Fatal("an action with the same id and a different digest was accepted")
	}
	sameStep := storedReadAction(t, run, 1)
	if err := recorder.StoreAction(ctx, sameStep); err == nil {
		t.Fatal("a second action for the same step was accepted")
	}
}

func TestRecorderWritesADenialWithoutAnActionRow(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	recorder := NewPostgresRecorder(pool)
	run := seedRun(t, pool)
	decision := Decision{Outcome: OutcomeDeny, ReasonCode: ReasonInvalidArguments, ActionID: testdb.ID(t)}
	if err := recorder.RecordDecision(ctx, run, decision); err != nil {
		t.Fatalf("record denial: %v", err)
	}
	var reason string
	var actionID *string
	if err := pool.QueryRow(ctx,
		`SELECT reason_code, action_id::text FROM runtime.audit_events WHERE run_id = $1 AND event_type = $2`,
		run.RunID, decisionEventType).Scan(&reason, &actionID); err != nil {
		t.Fatal(err)
	}
	if reason != string(ReasonInvalidArguments) || actionID != nil {
		t.Fatalf("reason %q, action %v; want invalid_arguments and no action reference", reason, actionID)
	}
}
