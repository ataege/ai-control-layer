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
		mustScan(t, rows, &value.class, &value.outcome, &value.rule, &value.source, &value.hasVerdict, &value.revision, &value.noAction)
		stored = append(stored, value)
	}
	mustFinishRows(t, rows)
	want := []row{
		{"deterministic", "block", "prompt_ignore_previous_v1", "", false, 3, true},
		// A record without its own revision takes the evaluation's.
		{"semantic", "pass", "", "fixture", true, 2, true},
	}
	if len(stored) != len(want) || stored[0] != want[0] || stored[1] != want[1] {
		t.Errorf("stored %+v\nwant   %+v", stored, want)
	}
}

// noFreeTextRecord is the semantic action check that made no model call (GO-77): semantic class,
// outcome not_applicable, no verdict source because there is no verdict to label.
func noFreeTextRecord() security.ControlRecord {
	return security.ControlRecord{Boundary: security.BoundaryActionProposal, Field: security.FieldActionProposalText,
		ControlClass: security.ClassSemantic, ControlID: security.ControlSemanticInjection, Outcome: security.OutcomeNotApplicable,
		ReasonCode: security.ReasonNoFreeTextArguments}
}

func TestPostgresNotApplicableSemanticRecordNeedsNoVerdictSource(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)
	keys := ControlKeys{OrganizationID: organizationID, RunID: passport.RunID, EvaluationID: testdb.ID(t),
		AdmissionCatalogRevisionID: 1, EvaluatedCatalogRevisionID: 2}
	err := repository.InTransaction(context.Background(), func(tx Tx) error {
		return tx.InsertControlRecords(context.Background(), keys, []security.ControlRecord{noFreeTextRecord()})
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	var class, outcome, reason string
	var source *string
	if err := outer.QueryRow(context.Background(), `SELECT control_class, outcome, reason_code, verdict_source
		FROM runtime.control_assessments WHERE evaluation_id = $1`, keys.EvaluationID).Scan(&class, &outcome, &reason, &source); err != nil {
		t.Fatal(err)
	}
	if class != "semantic" || outcome != "not_applicable" || reason != "no_free_text_arguments" || source != nil {
		t.Errorf("stored %s/%s/%s source %v", class, outcome, reason, source)
	}
}

// The table's relaxed check says exactly what the Go pre-write check says: only a semantic
// not_applicable row may lack its verdict source, and a deterministic row never has one.
func TestPostgresControlAssessmentCheckAllowsOnlyTheNotApplicableException(t *testing.T) {
	repository, outer := isolatedRepository(t)
	organizationID := testdb.ID(t)
	passport := samplePassport(t, organizationID)
	admit(t, repository, passport)
	insert := func(class, outcome string, source *string) error {
		_, err := outer.Exec(context.Background(), "SAVEPOINT assessment_check")
		if err != nil {
			t.Fatal(err)
		}
		_, insertErr := outer.Exec(context.Background(), `INSERT INTO runtime.control_assessments
			(organization_id, run_id, evaluation_id, boundary, control_class, control_id, outcome,
			 admission_catalog_revision_id, evaluated_catalog_revision_id, verdict_source)
			VALUES ($1, $2, $3, 'action_proposal', $4, 'semantic_injection', $5, 1, 1, $6)`,
			organizationID, passport.RunID, testdb.ID(t), class, outcome, source)
		if insertErr != nil {
			if _, err := outer.Exec(context.Background(), "ROLLBACK TO SAVEPOINT assessment_check"); err != nil {
				t.Fatal(err)
			}
		}
		return insertErr
	}
	fixture := "fixture"
	for name, testCase := range map[string]struct {
		class, outcome string
		source         *string
		allowed        bool
	}{
		"semantic not_applicable without source":  {"semantic", "not_applicable", nil, true},
		"semantic not_applicable with a source":   {"semantic", "not_applicable", &fixture, true},
		"semantic pass with a source":             {"semantic", "pass", &fixture, true},
		"semantic pass without a source":          {"semantic", "pass", nil, false},
		"semantic block without a source":         {"semantic", "block", nil, false},
		"deterministic not_applicable no source":  {"deterministic", "not_applicable", nil, true},
		"deterministic pass with a source":        {"deterministic", "pass", &fixture, false},
		"deterministic not_applicable one source": {"deterministic", "not_applicable", &fixture, false},
	} {
		err := insert(testCase.class, testCase.outcome, testCase.source)
		if (err == nil) != testCase.allowed {
			t.Errorf("%s: allowed = %v, error = %v", name, testCase.allowed, err)
		}
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
	notApplicableWithVerdict := noFreeTextRecord()
	notApplicableWithVerdict.Verdict = &security.Verdict{Score: 0.5}
	notApplicableWithCall := noFreeTextRecord()
	notApplicableWithCall.SecurityModelCallID = "1f2e3d4c-5b6a-4789-8a0b-c1d2e3f4a5b6"
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
		"not_applicable semantic record with a verdict": func() error {
			return tx.InsertControlRecords(context.Background(), keys, []security.ControlRecord{notApplicableWithVerdict})
		},
		"not_applicable semantic record with a model call": func() error {
			return tx.InsertControlRecords(context.Background(), keys, []security.ControlRecord{notApplicableWithCall})
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
