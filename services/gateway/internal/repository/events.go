package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"unicode/utf8"

	"starter/services/gateway/internal/contracts"
)

// maximumSafeMessageLength matches the X-12 schema bound on maskedSummary.safeMessage.
const maximumSafeMessageLength = 512

// maximumIdentifierLength matches the X-12 schema bound on identifier fields.
const maximumIdentifierLength = 256

// NewEvent is a safe event to append (X-12). It carries masked metadata only: never raw notes,
// secrets, model requests, classifier reasoning or review content.
type NewEvent struct {
	OrganizationID    string
	RunID             *string
	ActionID          *string
	EventType         contracts.EventType
	Decision          *contracts.EventDecision
	ReasonCode        *contracts.ReasonCode
	CatalogRevisionID *int64
	MaskedSummary     contracts.MaskedSummary
}

// Closed value sets of the masked summary fields that are plain strings in the Go type.
var (
	summaryPurposes        = []string{"agent", "security"}
	summaryClassifications = []string{"internal_only", "vendor_shareable"}
	summaryLineageChecks   = []string{"passed", "failed", "missing"}
	summaryEffects         = []string{"none", "read", "report_created", "outbox_message_queued"}
)

// AppendEvent validates the event against the X-12 contract and inserts it in this
// transaction, so it commits with the state change that produced it or not at all.
func (tx Tx) AppendEvent(ctx context.Context, event NewEvent) (contracts.SafeEvent, error) {
	if ctx == nil || !validEvent(event) {
		return contracts.SafeEvent{}, ErrInvalid
	}
	summary, err := json.Marshal(event.MaskedSummary)
	if err != nil {
		return contracts.SafeEvent{}, ErrInvalid
	}
	var decision, reason *string
	if event.Decision != nil {
		decisionText := string(*event.Decision)
		decision = &decisionText
	}
	if event.ReasonCode != nil {
		reasonText := string(*event.ReasonCode)
		reason = &reasonText
	}
	stored := contracts.SafeEvent{
		OrganizationID:    event.OrganizationID,
		RunID:             event.RunID,
		ActionID:          event.ActionID,
		EventType:         event.EventType,
		Decision:          event.Decision,
		ReasonCode:        event.ReasonCode,
		CatalogRevisionID: event.CatalogRevisionID,
		MaskedSummary:     event.MaskedSummary,
	}
	var eventID int64
	err = tx.transaction.QueryRow(ctx, `INSERT INTO runtime.audit_events
		(organization_id, run_id, action_id, event_type, decision, reason_code, catalog_revision_id, masked_summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, occurred_at`,
		event.OrganizationID, event.RunID, event.ActionID, string(event.EventType), decision, reason,
		event.CatalogRevisionID, summary).Scan(&eventID, &stored.OccurredAt)
	if err != nil {
		// A foreign-key failure (an unknown run or action of this organization) lands here too.
		return contracts.SafeEvent{}, ErrUnavailable
	}
	stored.EventID = strconv.FormatInt(eventID, 10)
	stored.OccurredAt = stored.OccurredAt.UTC()
	return stored, nil
}

func validEvent(event NewEvent) bool {
	if !validUUID(event.OrganizationID) || !event.EventType.Valid() {
		return false
	}
	if event.RunID != nil && !validUUID(*event.RunID) {
		return false
	}
	// An event that names an action also names its run (audit_events_action_needs_run).
	if event.ActionID != nil && (event.RunID == nil || !validUUID(*event.ActionID)) {
		return false
	}
	if event.Decision != nil && !event.Decision.Valid() {
		return false
	}
	if event.ReasonCode != nil && !event.ReasonCode.Valid() {
		return false
	}
	if event.CatalogRevisionID != nil && *event.CatalogRevisionID <= 0 {
		return false
	}
	return validSummary(event.MaskedSummary)
}

func validSummary(summary contracts.MaskedSummary) bool {
	return optionalOneOf(summary.Purpose, summaryPurposes) &&
		optionalOneOf(summary.Classification, summaryClassifications) &&
		optionalOneOf(summary.LineageCheck, summaryLineageChecks) &&
		optionalOneOf(summary.Effect, summaryEffects) &&
		(summary.AdmissionCatalogRevisionID == nil || *summary.AdmissionCatalogRevisionID > 0) &&
		optionalIdentifier(summary.MatchedRule) && optionalIdentifier(summary.FeedRevision) &&
		optionalIdentifier(summary.ReplaySource) &&
		(summary.ReportID == nil || validUUID(*summary.ReportID)) &&
		(summary.Template == nil || summary.Template.Valid()) &&
		(summary.AlternativeTemplate == nil || summary.AlternativeTemplate.Valid()) &&
		(summary.SafeMessage == nil || (utf8.ValidString(*summary.SafeMessage) &&
			utf8.RuneCountInString(*summary.SafeMessage) <= maximumSafeMessageLength))
}

func optionalOneOf(value *string, allowed []string) bool {
	if value == nil {
		return true
	}
	for _, candidate := range allowed {
		if *value == candidate {
			return true
		}
	}
	return false
}

// optionalIdentifier mirrors the schema's identifier: 1 to 256 characters, no control characters.
func optionalIdentifier(value *string) bool {
	if value == nil {
		return true
	}
	if *value == "" || !utf8.ValidString(*value) || utf8.RuneCountInString(*value) > maximumIdentifierLength {
		return false
	}
	for _, character := range *value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}
