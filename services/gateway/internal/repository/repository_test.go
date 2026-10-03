package repository

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

func pointer[Value any](value Value) *Value { return &value }

// isolatedRepository opens the configured test database and returns a repository whose
// transactions are savepoints of one outer transaction, rolled back when the test ends, so no
// fixture row survives (passports reject DELETE by trigger).
func isolatedRepository(t *testing.T) (*Repository, pgx.Tx) {
	t.Helper()
	pool := testdb.Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	outer, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin outer transaction: %v", err)
	}
	t.Cleanup(func() { _ = outer.Rollback(context.Background()) })
	return New(outer), outer
}

func samplePassport(t *testing.T, organizationID string) contracts.Passport {
	runID := testdb.ID(t)
	issuedAt := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	return contracts.Passport{
		PassportID:                 testdb.ID(t),
		RunID:                      runID,
		OrganizationID:             organizationID,
		ActorID:                    testdb.ID(t),
		TaskVersion:                "reconcile_atlas_v1",
		AdmissionCatalogRevisionID: 1,
		IssuedAt:                   issuedAt,
		ExpiresAt:                  issuedAt.Add(15 * time.Minute),
		Scope: contracts.PassportScope{
			Tools:                 contracts.ToolNames,
			InvoiceIDs:            []string{"invoice_A01", "invoice_A02"},
			VendorIDs:             []string{"vendor_Atlas"},
			ReportTemplates:       contracts.ReportTemplates,
			ProjectionRules:       []string{"vendor_invoice_fields_v1"},
			RecipientReferences:   []string{"recipient:" + runID + ":vendor_Atlas"},
			AllowedModels:         []string{"qwen3.5:4b"},
			InternalNoteReadable:  true,
			ApprovalRequiredTools: []contracts.ToolName{contracts.ToolQueueReport},
		},
		Limits: contracts.PassportLimits{
			CallsTotal: 24, CallsAgent: 12, CallsSecurity: 12, TokensTotal: 20000,
			RequestTimeoutSeconds: 20, LocalMaxConcurrency: 2, ToolAttempts: 12, Corrections: 2,
			RunExpiryMinutes: 15,
		},
	}
}

func admit(t *testing.T, repository *Repository, passport contracts.Passport) string {
	t.Helper()
	jobID := testdb.ID(t)
	err := repository.InTransaction(context.Background(), func(tx Tx) error {
		return tx.InsertAdmission(context.Background(), passport, NewJob{ID: jobID, Kind: contracts.JobKindAgentStep})
	})
	if err != nil {
		t.Fatalf("admission insert: %v", err)
	}
	return jobID
}

func runEvent(organizationID, runID string, eventType contracts.EventType) NewEvent {
	return NewEvent{OrganizationID: organizationID, RunID: &runID, EventType: eventType}
}

func countRows(t *testing.T, outer pgx.Tx, sql string, arguments ...any) int {
	t.Helper()
	var count int
	if err := outer.QueryRow(context.Background(), sql, arguments...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func TestPostgresAdmissionStoresPassportRunAndJob(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	jobID := admit(t, repository, passport)

	state, err := repository.RunState(context.Background(), organizationID, passport.RunID)
	if err != nil {
		t.Fatalf("read run: %v", err)
	}
	if state.Status != contracts.RunQueued || state.TerminalReason != nil || state.PassportID != passport.PassportID {
		t.Errorf("unexpected run state %+v", state)
	}
	stored, err := repository.Passport(context.Background(), organizationID, passport.RunID)
	if err != nil {
		t.Fatalf("read passport: %v", err)
	}
	if !reflect.DeepEqual(stored, passport) {
		t.Errorf("stored passport differs\nwant %+v\ngot  %+v", passport, stored)
	}
	var status, kind string
	err = outer.QueryRow(context.Background(), "SELECT status, kind FROM runtime.jobs WHERE id = $1", jobID).Scan(&status, &kind)
	if err != nil || status != "queued" || kind != contracts.JobKindAgentStep {
		t.Errorf("job row: status %q kind %q err %v", status, kind, err)
	}
}

func TestPostgresAdmissionCommitsAllOrNothing(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	injected := errors.New("injected failure after the inserts")
	err := repository.InTransaction(context.Background(), func(tx Tx) error {
		if err := tx.InsertAdmission(context.Background(), passport, NewJob{ID: testdb.ID(t), Kind: contracts.JobKindAgentStep}); err != nil {
			return err
		}
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatalf("expected the injected error, got %v", err)
	}
	for _, table := range []string{"runtime.passports", "runtime.runs", "runtime.jobs"} {
		if count := countRows(t, outer, "SELECT count(*) FROM "+table+" WHERE organization_id = $1", organizationID); count != 0 {
			t.Errorf("%s kept %d rows after rollback", table, count)
		}
	}

	// A second admission reusing a stored run id fails as a whole and leaves no second passport.
	admit(t, repository, passport)
	duplicate := passport
	duplicate.PassportID = testdb.ID(t)
	err = repository.InTransaction(context.Background(), func(tx Tx) error {
		return tx.InsertAdmission(context.Background(), duplicate, NewJob{ID: testdb.ID(t), Kind: contracts.JobKindAgentStep})
	})
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("duplicate run admission: got %v", err)
	}
	if count := countRows(t, outer, "SELECT count(*) FROM runtime.passports WHERE organization_id = $1", organizationID); count != 1 {
		t.Errorf("expected exactly one passport, found %d", count)
	}
}

func transition(t *testing.T, repository *Repository, organizationID, runID string, to contracts.RunStatus, reason *contracts.ReasonCode, event NewEvent) (contracts.RunState, error) {
	t.Helper()
	var state contracts.RunState
	err := repository.InTransaction(context.Background(), func(tx Tx) error {
		var transitionErr error
		state, transitionErr = tx.TransitionRun(context.Background(), RunTransition{
			OrganizationID: organizationID, RunID: runID, To: to, Reason: reason, Event: event,
		})
		return transitionErr
	})
	return state, err
}

func TestPostgresRunTransitionsAreGuarded(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)
	runID := passport.RunID
	eventCount := func() int {
		return countRows(t, outer, "SELECT count(*) FROM runtime.audit_events WHERE run_id = $1", runID)
	}

	if _, err := transition(t, repository, organizationID, runID, contracts.RunCompleted, nil,
		runEvent(organizationID, runID, contracts.EventRunCompleted)); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("queued to completed: got %v", err)
	}
	if eventCount() != 0 {
		t.Error("a refused transition wrote an event")
	}

	state, err := transition(t, repository, organizationID, runID, contracts.RunRunning, nil,
		runEvent(organizationID, runID, contracts.EventRunStarted))
	if err != nil || state.Status != contracts.RunRunning {
		t.Fatalf("queued to running: state %+v err %v", state, err)
	}
	if eventCount() != 1 {
		t.Errorf("expected one event, found %d", eventCount())
	}

	if _, err := transition(t, repository, organizationID, runID, contracts.RunPaused, nil,
		runEvent(organizationID, runID, contracts.EventRunPaused)); !errors.Is(err, ErrInvalid) {
		t.Errorf("pause without a reason: got %v", err)
	}
	if _, err := transition(t, repository, organizationID, runID, contracts.RunRunning, pointer(contracts.ReasonRunCancelled),
		runEvent(organizationID, runID, contracts.EventRunStarted)); !errors.Is(err, ErrInvalid) {
		t.Errorf("running with a reason: got %v", err)
	}

	state, err = transition(t, repository, organizationID, runID, contracts.RunPaused, pointer(contracts.ReasonAllowanceExhausted),
		runEvent(organizationID, runID, contracts.EventRunPaused))
	if err != nil || state.TerminalReason == nil || *state.TerminalReason != contracts.ReasonAllowanceExhausted {
		t.Fatalf("running to paused: state %+v err %v", state, err)
	}
	// Resuming clears the reason; a stop then names its own.
	state, err = transition(t, repository, organizationID, runID, contracts.RunRunning, nil,
		runEvent(organizationID, runID, contracts.EventRunStarted))
	if err != nil || state.TerminalReason != nil {
		t.Fatalf("paused to running: state %+v err %v", state, err)
	}
	if _, err = transition(t, repository, organizationID, runID, contracts.RunStopped, pointer(contracts.ReasonRunCancelled),
		runEvent(organizationID, runID, contracts.EventRunStopped)); err != nil {
		t.Fatalf("running to stopped: %v", err)
	}
	for _, target := range contracts.RunStatuses {
		reason := (*contracts.ReasonCode)(nil)
		if target.NeedsReason() {
			reason = pointer(contracts.ReasonRunCancelled)
		}
		if _, err := transition(t, repository, organizationID, runID, target, reason,
			runEvent(organizationID, runID, contracts.EventRunStarted)); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("stopped to %s: got %v", target, err)
		}
	}
	final, err := repository.RunState(context.Background(), organizationID, runID)
	if err != nil || final.Status != contracts.RunStopped {
		t.Errorf("terminal state changed: %+v %v", final, err)
	}
}

func TestPostgresStateChangeAndEventCommitTogether(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)
	runID := passport.RunID

	// The event names an action that does not exist, so its insert fails after the update.
	event := runEvent(organizationID, runID, contracts.EventRunStarted)
	event.ActionID = pointer(testdb.ID(t))
	if _, err := transition(t, repository, organizationID, runID, contracts.RunRunning, nil, event); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected the event insert to fail, got %v", err)
	}
	state, err := repository.RunState(context.Background(), organizationID, runID)
	if err != nil || state.Status != contracts.RunQueued {
		t.Errorf("state changed without its event: %+v %v", state, err)
	}
	if count := countRows(t, outer, "SELECT count(*) FROM runtime.audit_events WHERE run_id = $1", runID); count != 0 {
		t.Errorf("found %d events after the rollback", count)
	}
}

func TestPostgresOtherOrganizationSeesNothing(t *testing.T) {
	repository, _ := isolatedRepository(t)
	organizationID := testdb.ID(t)
	otherOrganizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)

	if _, err := repository.RunState(context.Background(), otherOrganizationID, passport.RunID); !errors.Is(err, ErrNotFound) {
		t.Errorf("run read across organizations: %v", err)
	}
	if _, err := repository.Passport(context.Background(), otherOrganizationID, passport.RunID); !errors.Is(err, ErrNotFound) {
		t.Errorf("passport read across organizations: %v", err)
	}
	if _, err := transition(t, repository, otherOrganizationID, passport.RunID, contracts.RunRunning, nil,
		runEvent(otherOrganizationID, passport.RunID, contracts.EventRunStarted)); !errors.Is(err, ErrNotFound) {
		t.Errorf("transition across organizations: %v", err)
	}
	state, err := repository.RunState(context.Background(), organizationID, passport.RunID)
	if err != nil || state.Status != contracts.RunQueued {
		t.Errorf("the owning organization's run changed: %+v %v", state, err)
	}
}

func TestPostgresAppendEventReturnsOrderedCursor(t *testing.T) {
	repository, _ := isolatedRepository(t)
	organizationID := testdb.ID(t)
	var first, second contracts.SafeEvent
	err := repository.InTransaction(context.Background(), func(tx Tx) error {
		var err error
		event := NewEvent{
			OrganizationID: organizationID,
			EventType:      contracts.EventAdmissionRejected,
			Decision:       pointer(contracts.DecisionDeny),
			ReasonCode:     pointer(contracts.ReasonResourceOutOfScope),
			MaskedSummary:  contracts.MaskedSummary{Effect: pointer("none"), SafeMessage: pointer("Requested invoice is outside the task's authority.")},
		}
		if first, err = tx.AppendEvent(context.Background(), event); err != nil {
			return err
		}
		second, err = tx.AppendEvent(context.Background(), event)
		return err
	})
	if err != nil {
		t.Fatalf("append events: %v", err)
	}
	if first.EventID == "" || first.EventID == second.EventID || first.RunID != nil || first.OccurredAt.Location() != time.UTC {
		t.Errorf("unexpected events %+v %+v", first, second)
	}
}

func TestInvalidInputIsRejectedBeforeDatabaseWork(t *testing.T) {
	organizationID := "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01"
	runID := "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b"
	// A Tx without a transaction proves validation runs first: any database call would panic.
	var tx Tx
	ctx := context.Background()
	invalidEvents := map[string]NewEvent{
		"unknown type":              {OrganizationID: organizationID, EventType: "run.exploded"},
		"action without run":        {OrganizationID: organizationID, ActionID: pointer(runID), EventType: contracts.EventActionDenied},
		"unknown reason":            {OrganizationID: organizationID, EventType: contracts.EventActionDenied, ReasonCode: pointer(contracts.ReasonCode("nope"))},
		"unknown purpose":           {OrganizationID: organizationID, EventType: contracts.EventModelCompleted, MaskedSummary: contracts.MaskedSummary{Purpose: pointer("tool")}},
		"control character in rule": {OrganizationID: organizationID, EventType: contracts.EventControlEvaluated, MaskedSummary: contracts.MaskedSummary{MatchedRule: pointer("rule\nname")}},
		"bad organization":          {OrganizationID: "demo_org", EventType: contracts.EventRunQueued},
		"zero catalog":              {OrganizationID: organizationID, EventType: contracts.EventRunQueued, CatalogRevisionID: pointer(int64(0))},
		"unknown effect":            {OrganizationID: organizationID, EventType: contracts.EventActionSucceeded, MaskedSummary: contracts.MaskedSummary{Effect: pointer("email_sent")}},
		"unknown alternative":       {OrganizationID: organizationID, EventType: contracts.EventActionDenied, MaskedSummary: contracts.MaskedSummary{AlternativeTemplate: pointer(contracts.ReportTemplate("public_v1"))}},
	}
	for name, event := range invalidEvents {
		if _, err := tx.AppendEvent(ctx, event); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	otherRunID := "1f2e3d4c-5b6a-4789-8a0b-c1d2e3f4a5b6"
	if _, err := tx.TransitionRun(ctx, RunTransition{OrganizationID: organizationID, RunID: runID, To: contracts.RunRunning,
		Event: runEvent(organizationID, otherRunID, contracts.EventRunStarted)}); !errors.Is(err, ErrInvalid) {
		t.Errorf("event for another run: got %v", err)
	}
	if err := tx.InsertAdmission(ctx, contracts.Passport{}, NewJob{}); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty passport: got %v", err)
	}
}

func TestTerminalStatusesHaveNoOutgoingTransitions(t *testing.T) {
	for _, terminal := range []contracts.RunStatus{contracts.RunCompleted, contracts.RunFailed, contracts.RunStopped} {
		if _, hasTargets := allowedRunTransitions[terminal]; hasTargets {
			t.Errorf("%s must be terminal", terminal)
		}
	}
	if sources := sourceStatusesFor(contracts.RunQueued); len(sources) != 0 {
		t.Errorf("no run may return to queued, but %v can", sources)
	}
	if !slices.Contains(sourceStatusesFor(contracts.RunRunning), string(contracts.RunAwaitingApproval)) {
		t.Error("a run must resume after an approval wait")
	}
}

func TestPostgresEnqueueJobStaysInsideTheOrganization(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)

	var jobID string
	err := repository.InTransaction(context.Background(), func(tx Tx) error {
		var enqueueErr error
		jobID, enqueueErr = tx.EnqueueJob(context.Background(), organizationID, passport.RunID, contracts.JobKindAgentStep)
		return enqueueErr
	})
	if err != nil || !validUUID(jobID) {
		t.Fatalf("enqueue: id %q err %v", jobID, err)
	}
	var status, kind string
	if err := outer.QueryRow(context.Background(), "SELECT status, kind FROM runtime.jobs WHERE id = $1", jobID).Scan(&status, &kind); err != nil ||
		status != "queued" || kind != contracts.JobKindAgentStep {
		t.Errorf("job row: %q %q %v", status, kind, err)
	}

	otherOrganizationID := testdb.ID(t)
	err = repository.InTransaction(context.Background(), func(tx Tx) error {
		_, enqueueErr := tx.EnqueueJob(context.Background(), otherOrganizationID, passport.RunID, contracts.JobKindAgentStep)
		return enqueueErr
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("enqueue for another organization's run: %v", err)
	}
	if count := countRows(t, outer, "SELECT count(*) FROM runtime.jobs WHERE run_id = $1", passport.RunID); count != 2 {
		t.Errorf("expected the admission job and one continuation, found %d", count)
	}
}

func TestEnqueueJobRejectsUnknownKinds(t *testing.T) {
	var tx Tx
	_, err := tx.EnqueueJob(context.Background(), "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01", "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b", "shell")
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown kind: %v", err)
	}
}
