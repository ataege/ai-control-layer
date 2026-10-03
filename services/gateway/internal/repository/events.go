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
	// A run's events must commit in cursor order, or a reader paging by id could pass an event
	// that commits later. Locking the run row until commit serializes the run's event writers, so
	// each new id is allocated only after earlier writers of the run have committed. NO KEY UPDATE
	// (the lock a status UPDATE takes) does not conflict with the KEY SHARE lock that inserting a
	// child row such as an action takes, so a writer that did both cannot deadlock with another.
	if event.RunID != nil {
		var locked int
		err := tx.transaction.QueryRow(ctx, `SELECT 1 FROM runtime.runs WHERE id = $1 AND organization_id = $2 FOR NO KEY UPDATE`,
			*event.RunID, event.OrganizationID).Scan(&locked)
		if err != nil {
			return contracts.SafeEvent{}, storageError(err)
		}
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

// maximumEventPage bounds one cursor read.
const maximumEventPage = 500

// RunEvents returns the run's events after the cursor (an event id, 0 for the start) in order,
// at most limit of them. Run-scoped events commit in id order (see AppendEvent), so paging by the
// last returned id never skips one. Another organization's run is ErrNotFound.
func (repository *Repository) RunEvents(ctx context.Context, organizationID, runID string, afterEventID int64, limit int) ([]contracts.SafeEvent, error) {
	if ctx == nil || !validUUID(organizationID) || !validUUID(runID) || afterEventID < 0 || limit < 1 || limit > maximumEventPage {
		return nil, ErrInvalid
	}
	var events []contracts.SafeEvent
	err := repository.InTransaction(ctx, func(tx Tx) error {
		if _, err := tx.readRunState(ctx, organizationID, runID); err != nil {
			return err
		}
		rows, err := tx.transaction.Query(ctx, `SELECT id, organization_id::text, run_id::text, action_id::text,
			event_type, decision, reason_code, catalog_revision_id, masked_summary, occurred_at
			FROM runtime.audit_events
			WHERE organization_id = $1 AND run_id = $2 AND id > $3
			ORDER BY id LIMIT $4`, organizationID, runID, afterEventID, limit)
		if err != nil {
			return ErrUnavailable
		}
		defer rows.Close()
		for rows.Next() {
			event, err := scanEvent(rows)
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		if rows.Err() != nil {
			return ErrUnavailable
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return nonNilEvents(events), nil
}

// scanEvent reads one stored event and re-checks it against X-12; a row outside the contract is
// ErrUnavailable rather than a partial or unchecked event.
func scanEvent(row rowScanner) (contracts.SafeEvent, error) {
	var eventID int64
	var eventType string
	var decision, reason *string
	var summary []byte
	event := contracts.SafeEvent{}
	err := row.Scan(&eventID, &event.OrganizationID, &event.RunID, &event.ActionID, &eventType, &decision,
		&reason, &event.CatalogRevisionID, &summary, &event.OccurredAt)
	if err != nil {
		return contracts.SafeEvent{}, ErrUnavailable
	}
	event.EventID = strconv.FormatInt(eventID, 10)
	event.EventType = contracts.EventType(eventType)
	if decision != nil {
		value := contracts.EventDecision(*decision)
		event.Decision = &value
	}
	if reason != nil {
		value := contracts.ReasonCode(*reason)
		event.ReasonCode = &value
	}
	if contracts.DecodeStrict(summary, &event.MaskedSummary) != nil {
		return contracts.SafeEvent{}, ErrUnavailable
	}
	event.OccurredAt = event.OccurredAt.UTC()
	if !ValidStoredEvent(event) {
		return contracts.SafeEvent{}, ErrUnavailable
	}
	return event, nil
}

func nonNilEvents(events []contracts.SafeEvent) []contracts.SafeEvent {
	if events == nil {
		return []contracts.SafeEvent{}
	}
	return events
}

// ValidStoredEvent reports whether an event read from runtime.audit_events fits X-12: a decimal
// cursor, an occurrence time and every value check AppendEvent applies. Readers that page events
// themselves (for example organization-wide) use it so no row outside the contract is served.
func ValidStoredEvent(event contracts.SafeEvent) bool {
	eventID, err := strconv.ParseInt(event.EventID, 10, 64)
	if err != nil || eventID <= 0 || strconv.FormatInt(eventID, 10) != event.EventID || event.OccurredAt.IsZero() {
		return false
	}
	return validEvent(NewEvent{OrganizationID: event.OrganizationID, RunID: event.RunID, ActionID: event.ActionID,
		EventType: event.EventType, Decision: event.Decision, ReasonCode: event.ReasonCode,
		CatalogRevisionID: event.CatalogRevisionID, MaskedSummary: event.MaskedSummary})
}
