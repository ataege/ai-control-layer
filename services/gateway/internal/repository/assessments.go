package repository

import (
	"context"
	"encoding/json"
	"math"

	"starter/services/gateway/internal/security"
)

// ControlKeys are what one evaluation's control records belong to.
type ControlKeys struct {
	OrganizationID string
	RunID          string
	// EvaluationID groups the records of one decision or inspection.
	EvaluationID string
	// ActionID is the stored action, or empty when none was stored (an evaluation of judge input).
	ActionID                   string
	AdmissionCatalogRevisionID int64
	// EvaluatedCatalogRevisionID applies to a record that does not carry its own revision.
	EvaluatedCatalogRevisionID int64
}

var (
	controlBoundaries = []security.Boundary{security.BoundaryModelInput, security.BoundaryToolResult, security.BoundaryActionProposal}
	controlOutcomes   = []security.Outcome{security.OutcomePass, security.OutcomeRedact, security.OutcomeBlock,
		security.OutcomeError, security.OutcomeNotApplicable}
)

// InsertControlRecords writes control decisions to runtime.control_assessments in this
// transaction: the one writer of that table (lead decision), so a decision and its evidence
// commit together. It stores ids, outcomes, codes, revisions and the bounded verdict only, never
// inspected text. Every record is checked against the table's rules before any write.
func (tx Tx) InsertControlRecords(ctx context.Context, keys ControlKeys, records []security.ControlRecord) error {
	if ctx == nil || !validControlKeys(keys) {
		return ErrInvalid
	}
	for _, record := range records {
		if !validControlRecord(record) {
			return ErrInvalid
		}
	}
	for _, record := range records {
		var verdict []byte
		if record.Verdict != nil {
			encoded, err := json.Marshal(record.Verdict)
			if err != nil {
				return ErrInvalid
			}
			verdict = encoded
		}
		evaluatedRevision := record.EvaluatedCatalogRevisionID
		if evaluatedRevision <= 0 {
			evaluatedRevision = keys.EvaluatedCatalogRevisionID
		}
		_, err := tx.transaction.Exec(ctx, `INSERT INTO runtime.control_assessments
			(organization_id, run_id, evaluation_id, action_id, security_model_call_id, boundary, control_class,
			 control_id, outcome, reason_code, admission_catalog_revision_id, evaluated_catalog_revision_id,
			 matched_rule_id, feed_revision, verdict_source, verdict)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			keys.OrganizationID, keys.RunID, keys.EvaluationID, optionalText(keys.ActionID),
			optionalText(record.SecurityModelCallID), string(record.Boundary), string(record.ControlClass),
			record.ControlID, string(record.Outcome), optionalText(record.ReasonCode),
			keys.AdmissionCatalogRevisionID, evaluatedRevision, optionalText(record.MatchedRuleID),
			optionalText(record.FeedRevision), optionalText(string(record.VerdictSource)), verdict)
		if err != nil {
			// A foreign-key failure (another organization's run, action or security call) lands here.
			return ErrUnavailable
		}
	}
	return nil
}

func validControlKeys(keys ControlKeys) bool {
	return validUUID(keys.OrganizationID) && validUUID(keys.RunID) && validUUID(keys.EvaluationID) &&
		(keys.ActionID == "" || validUUID(keys.ActionID)) && keys.AdmissionCatalogRevisionID > 0 &&
		keys.EvaluatedCatalogRevisionID > 0
}

// validControlRecord mirrors the table's checks: a semantic record names its verdict source and
// may carry a verdict and its security call; a deterministic one carries none of them.
func validControlRecord(record security.ControlRecord) bool {
	if !containsValue(controlBoundaries, record.Boundary) || !containsValue(controlOutcomes, record.Outcome) ||
		record.ControlID == "" || len(record.ControlID) > maximumIdentifierLength || record.EvaluatedCatalogRevisionID < 0 {
		return false
	}
	if record.SecurityModelCallID != "" && !validUUID(record.SecurityModelCallID) {
		return false
	}
	switch record.ControlClass {
	case security.ClassSemantic:
		if record.VerdictSource != security.VerdictLive && record.VerdictSource != security.VerdictFixture {
			return false
		}
		return record.Verdict == nil || (!math.IsNaN(record.Verdict.Score) && !math.IsInf(record.Verdict.Score, 0))
	case security.ClassDeterministic:
		return record.VerdictSource == "" && record.Verdict == nil && record.SecurityModelCallID == ""
	default:
		return false
	}
}

func containsValue[Value comparable](values []Value, candidate Value) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

// optionalText stores an empty string as NULL.
func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
