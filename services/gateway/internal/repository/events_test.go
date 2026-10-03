package repository

import (
	"context"
	"errors"
	"math/rand/v2"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

// removeCommittedRun deletes a committed fixture run. Passports reject DELETE by trigger, so the
// passport goes with triggers disabled for one transaction, which needs a superuser test
// database; otherwise it is left in place and logged.
func removeCommittedRun(t *testing.T, pool *pgxpool.Pool, passport contracts.Passport) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, statement := range []string{
		"DELETE FROM runtime.audit_events WHERE run_id = $1",
		"DELETE FROM runtime.jobs WHERE run_id = $1",
		"DELETE FROM runtime.runs WHERE id = $1",
	} {
		if _, err := pool.Exec(ctx, statement, passport.RunID); err != nil {
			t.Errorf("cleanup failed: %v", err)
			return
		}
	}
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Error("cleanup could not begin")
		return
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err = transaction.Exec(ctx, "SET LOCAL session_replication_role = replica"); err == nil {
		_, err = transaction.Exec(ctx, "DELETE FROM runtime.passports WHERE id = $1", passport.PassportID)
	}
	if err == nil {
		err = transaction.Commit(ctx)
	}
	if err != nil {
		t.Logf("passport fixture %s left in place (removing it needs a superuser)", passport.PassportID)
	}
}

// Concurrent writers of one run commit their events while a reader pages by cursor; every
// event must be read exactly once, in order, although ids are allocated before commit.
func TestPostgresConcurrentRunEventsAreReadWithoutGaps(t *testing.T) {
	pool := testdb.Open(t)
	repository := New(pool)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)
	t.Cleanup(func() { removeCommittedRun(t, pool, passport) })

	const writers, eventsPerWriter = 4, 15
	var writersDone sync.WaitGroup
	writeErrors := make(chan error, writers*eventsPerWriter)
	for range writers {
		writersDone.Add(1)
		go func() {
			defer writersDone.Done()
			for range eventsPerWriter {
				writeErrors <- repository.InTransaction(context.Background(), func(tx Tx) error {
					if _, err := tx.AppendEvent(context.Background(), runEvent(organizationID, passport.RunID, contracts.EventModelCompleted)); err != nil {
						return err
					}
					// Hold the transaction open briefly so commits and id allocation interleave.
					time.Sleep(time.Duration(rand.IntN(3)) * time.Millisecond)
					return nil
				})
			}
		}()
	}
	allWritten := make(chan struct{})
	go func() { writersDone.Wait(); close(allWritten) }()

	var readIDs []int64
	var cursor int64
	readPage := func() {
		events, err := repository.RunEvents(context.Background(), organizationID, passport.RunID, cursor, 7)
		if err != nil {
			t.Fatalf("read events: %v", err)
		}
		for _, event := range events {
			eventID, _ := strconv.ParseInt(event.EventID, 10, 64)
			readIDs = append(readIDs, eventID)
			cursor = eventID
		}
	}
	for finished := false; !finished; {
		select {
		case <-allWritten:
			finished = true
		default:
			readPage()
		}
	}
	for previous := -1; previous != len(readIDs); {
		previous = len(readIDs)
		readPage()
	}
	close(writeErrors)
	for err := range writeErrors {
		if err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	var storedIDs []int64
	rows, err := pool.Query(context.Background(), "SELECT id FROM runtime.audit_events WHERE run_id = $1 ORDER BY id", passport.RunID)
	if err != nil {
		t.Fatalf("list stored events: %v", err)
	}
	for rows.Next() {
		var eventID int64
		_ = rows.Scan(&eventID)
		storedIDs = append(storedIDs, eventID)
	}
	rows.Close()
	if len(storedIDs) != writers*eventsPerWriter || !reflect.DeepEqual(readIDs, storedIDs) {
		t.Errorf("the cursor reader missed or repeated events: read %d, stored %d", len(readIDs), len(storedIDs))
	}
}

func TestPostgresRunEventsCarryEveryLink(t *testing.T) {
	repository, _ := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)
	reportID := testdb.ID(t)
	event := NewEvent{
		OrganizationID:    organizationID,
		RunID:             &passport.RunID,
		EventType:         contracts.EventReportExportDenied,
		Decision:          pointer(contracts.DecisionDeny),
		ReasonCode:        pointer(contracts.ReasonReportExportRestricted),
		CatalogRevisionID: pointer(int64(2)),
		MaskedSummary: contracts.MaskedSummary{
			Purpose:                    pointer("security"),
			AdmissionCatalogRevisionID: pointer(int64(1)),
			MatchedRule:                pointer("prompt_ignore_previous_v1"),
			FeedRevision:               pointer("feed_v1"),
			ReportID:                   &reportID,
			Template:                   pointer(contracts.TemplateInternalInvestigation),
			Classification:             pointer("internal_only"),
			LineageCheck:               pointer("passed"),
			Effect:                     pointer("none"),
			ReplaySource:               pointer("labelled_replay_v1"),
			AlternativeTemplate:        pointer(contracts.TemplateVendorReconciliation),
			SafeMessage:                pointer("The report inherits an Internal only restriction."),
		},
	}
	var appended contracts.SafeEvent
	if err := repository.InTransaction(context.Background(), func(tx Tx) error {
		var err error
		appended, err = tx.AppendEvent(context.Background(), event)
		return err
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	events, err := repository.RunEvents(context.Background(), organizationID, passport.RunID, 0, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("read: %v %v", events, err)
	}
	if !reflect.DeepEqual(events[0], appended) {
		t.Errorf("read event differs from the appended one\nappended %+v\nread     %+v", appended, events[0])
	}

	empty, err := repository.RunEvents(context.Background(), organizationID, passport.RunID, mustParse(t, appended.EventID), 10)
	if err != nil || len(empty) != 0 {
		t.Errorf("after the last cursor: %v %v", empty, err)
	}
}

func mustParse(t *testing.T, value string) int64 {
	t.Helper()
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return parsed
}

func TestPostgresEventsStayInsideTheOrganization(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID, otherOrganizationID := testdb.ID(t), testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)

	if _, err := repository.RunEvents(context.Background(), otherOrganizationID, passport.RunID, 0, 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("read another organization's run: %v", err)
	}
	err := repository.InTransaction(context.Background(), func(tx Tx) error {
		_, appendErr := tx.AppendEvent(context.Background(), runEvent(otherOrganizationID, passport.RunID, contracts.EventRunStarted))
		return appendErr
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("event for another organization's run: %v", err)
	}
	if count := countRows(t, outer, "SELECT count(*) FROM runtime.audit_events WHERE run_id = $1", passport.RunID); count != 0 {
		t.Errorf("found %d events", count)
	}
}

// A row written around the validated writer with an extra summary key is never served.
func TestPostgresRunEventsFailClosedOnARowOutsideTheContract(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)
	if _, err := outer.Exec(context.Background(), `INSERT INTO runtime.audit_events (organization_id, run_id, event_type, masked_summary)
		VALUES ($1, $2, 'run.started', '{"rawNote": "confidential investigation text"}')`, organizationID, passport.RunID); err != nil {
		t.Fatalf("insert raw row: %v", err)
	}
	if _, err := repository.RunEvents(context.Background(), organizationID, passport.RunID, 0, 10); !errors.Is(err, ErrUnavailable) {
		t.Errorf("a row outside X-12 was served: %v", err)
	}
}

func TestPostgresJoinUsesTheCallersTransaction(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)
	callerTransaction, err := outer.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := Join(callerTransaction).AppendEvent(context.Background(), runEvent(organizationID, passport.RunID, contracts.EventActionProposed)); err != nil {
		t.Fatalf("append through Join: %v", err)
	}
	_ = callerTransaction.Rollback(context.Background())
	if count := countRows(t, outer, "SELECT count(*) FROM runtime.audit_events WHERE run_id = $1", passport.RunID); count != 0 {
		t.Errorf("the event outlived the caller's rollback: %d", count)
	}
}

func TestRunEventsRejectsInvalidPagesBeforeDatabaseWork(t *testing.T) {
	repository := New(nil)
	organizationID, runID := "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01", "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b"
	for name, call := range map[string]func() error{
		"negative cursor": func() error {
			_, err := repository.RunEvents(context.Background(), organizationID, runID, -1, 10)
			return err
		},
		"zero limit": func() error {
			_, err := repository.RunEvents(context.Background(), organizationID, runID, 0, 0)
			return err
		},
		"huge limit": func() error {
			_, err := repository.RunEvents(context.Background(), organizationID, runID, 0, 501)
			return err
		},
		"bad run id": func() error {
			_, err := repository.RunEvents(context.Background(), organizationID, "run-1", 0, 10)
			return err
		},
	} {
		if err := call(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestValidStoredEventChecksTheCursorAndTheContract(t *testing.T) {
	runID := "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b"
	valid := contracts.SafeEvent{EventID: "42", OrganizationID: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01", RunID: &runID,
		EventType: contracts.EventRunStarted, OccurredAt: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)}
	if !ValidStoredEvent(valid) {
		t.Fatal("a valid event was refused")
	}
	for name, change := range map[string]func(event *contracts.SafeEvent){
		"cursor with a leading zero": func(event *contracts.SafeEvent) { event.EventID = "042" },
		"cursor not a number":        func(event *contracts.SafeEvent) { event.EventID = "abc" },
		"no occurrence time":         func(event *contracts.SafeEvent) { event.OccurredAt = time.Time{} },
		"unknown event type":         func(event *contracts.SafeEvent) { event.EventType = "run.exploded" },
		"unknown summary effect":     func(event *contracts.SafeEvent) { event.MaskedSummary.Effect = pointer("email_sent") },
	} {
		event := valid
		change(&event)
		if ValidStoredEvent(event) {
			t.Errorf("%s was accepted", name)
		}
	}
}
