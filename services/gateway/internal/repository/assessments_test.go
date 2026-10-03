package repository

import (
	"context"
	"errors"
	"testing"

	"starter/services/gateway/internal/security"
	"starter/services/gateway/internal/testdb"
)

func deterministicRecord() security.ControlRecord {
	return security.ControlRecord{Boundary: security.BoundaryToolResult, Field: security.FieldToolResultText,
		ControlClass: security.ClassDeterministic, ControlID: security.ControlSignatureMatch, Outcome: security.OutcomeBlock,
		ReasonCode: "signature_match", MatchedRuleID: "prompt_ignore_previous_v1", FeedRevision: "feed_v1", EvaluatedCatalogRevisionID: 3}
}

func semanticRecord() security.ControlRecord {
	return security.ControlRecord{Boundary: security.BoundaryModelInput, Field: security.FieldModelInputText,
		ControlClass: security.ClassSemantic, ControlID: security.ControlSemanticInjection, Outcome: security.OutcomePass,
		VerdictSource: security.VerdictFixture, Verdict: &security.Verdict{RiskCategory: "none", Score: 0.1, ReasonCode: "none"}}
}

func TestPostgresControlRecordsAreWrittenWithTheirKeys(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)
	keys := ControlKeys{OrganizationID: organizationID, RunID: passport.RunID, EvaluationID: testdb.ID(t),
		AdmissionCatalogRevisionID: 1, EvaluatedCatalogRevisionID: 2}
	err := repository.InTransaction(context.Background(), func(tx Tx) error {
		return tx.InsertControlRecords(context.Background(), keys, []security.ControlRecord{deterministicRecord(), semanticRecord()})
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	rows, err := outer.Query(context.Background(), `SELECT control_class, outcome, coalesce(matched_rule_id, ''),
		coalesce(verdict_source, ''), verdict IS NOT NULL, evaluated_catalog_revision_id, action_id IS NULL
		FROM runtime.control_assessments WHERE evaluation_id = $1 ORDER BY id`, keys.EvaluationID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type row struct {
		class, outcome, rule, source string
		hasVerdict                   bool
		revision                     int64
		noAction                     bool
	}
	var stored []row
	for rows.Next() {
		var value row
		_ = rows.Scan(&value.class, &value.outcome, &value.rule, &value.source, &value.hasVerdict, &value.revision, &value.noAction)
		stored = append(stored, value)
	}
	want := []row{
		{"deterministic", "block", "prompt_ignore_previous_v1", "", false, 3, true},
		// A record without its own revision takes the evaluation's.
		{"semantic", "pass", "", "fixture", true, 2, true},
	}
	if len(stored) != len(want) || stored[0] != want[0] || stored[1] != want[1] {
		t.Errorf("stored %+v\nwant   %+v", stored, want)
	}
}

func TestPostgresControlRecordsOfAnotherOrganizationAreRefused(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)
	keys := ControlKeys{OrganizationID: testdb.ID(t), RunID: passport.RunID, EvaluationID: testdb.ID(t),
		AdmissionCatalogRevisionID: 1, EvaluatedCatalogRevisionID: 1}
	err := repository.InTransaction(context.Background(), func(tx Tx) error {
		return tx.InsertControlRecords(context.Background(), keys, []security.ControlRecord{deterministicRecord()})
	})
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("got %v, want ErrUnavailable", err)
	}
	if count := countRows(t, outer, "SELECT count(*) FROM runtime.control_assessments WHERE run_id = $1", passport.RunID); count != 0 {
		t.Errorf("%d rows written", count)
	}
}

func TestInvalidControlRecordsAreRefusedBeforeDatabaseWork(t *testing.T) {
	keys := ControlKeys{OrganizationID: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01", RunID: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
		EvaluationID: "9e8d7c6b-5a49-4382-a716-5f4e3d2c1b0a", AdmissionCatalogRevisionID: 1, EvaluatedCatalogRevisionID: 1}
	semanticWithoutSource := semanticRecord()
	semanticWithoutSource.VerdictSource = ""
	deterministicWithVerdict := deterministicRecord()
	deterministicWithVerdict.Verdict = &security.Verdict{Score: 0.5}
	deterministicWithCall := deterministicRecord()
	deterministicWithCall.SecurityModelCallID = "1f2e3d4c-5b6a-4789-8a0b-c1d2e3f4a5b6"
	unknownBoundary := deterministicRecord()
	unknownBoundary.Boundary = "email"
	unknownClass := deterministicRecord()
	unknownClass.ControlClass = "heuristic"
	missingKeys := keys
	missingKeys.AdmissionCatalogRevisionID = 0
	var tx Tx // no transaction: a database call would panic, so validation must refuse first
	for name, call := range map[string]func() error{
		"semantic without verdict source": func() error {
			return tx.InsertControlRecords(context.Background(), keys, []security.ControlRecord{semanticWithoutSource})
		},
		"deterministic with a verdict": func() error {
			return tx.InsertControlRecords(context.Background(), keys, []security.ControlRecord{deterministicWithVerdict})
		},
		"deterministic with a model call": func() error {
			return tx.InsertControlRecords(context.Background(), keys, []security.ControlRecord{deterministicWithCall})
		},
		"unknown boundary": func() error {
			return tx.InsertControlRecords(context.Background(), keys, []security.ControlRecord{unknownBoundary})
		},
		"unknown class": func() error {
			return tx.InsertControlRecords(context.Background(), keys, []security.ControlRecord{unknownClass})
		},
		"missing admission revision": func() error { return tx.InsertControlRecords(context.Background(), missingKeys, nil) },
	} {
		if err := call(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
