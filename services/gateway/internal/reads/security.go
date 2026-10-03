package reads

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
)

// AssessmentRecord is one control assessment as the security summary and the audit export read
// it (GO-83, the Go side's draft until the shared contract lands it). It carries stable codes and
// revisions only: never inspected text, model requests or classifier reasoning.
type AssessmentRecord struct {
	AssessmentID               string                `json:"assessmentId"`
	RunID                      string                `json:"runId"`
	EvaluationID               string                `json:"evaluationId"`
	ActionID                   *string               `json:"actionId"`
	SecurityModelCallID        *string               `json:"securityModelCallId"`
	Boundary                   string                `json:"boundary"`
	ControlClass               string                `json:"controlClass"`
	ControlID                  string                `json:"controlId"`
	Outcome                    string                `json:"outcome"`
	ReasonCode                 *contracts.ReasonCode `json:"reasonCode"`
	AdmissionCatalogRevisionID int64                 `json:"admissionCatalogRevisionId"`
	EvaluatedCatalogRevisionID int64                 `json:"evaluatedCatalogRevisionId"`
	MatchedRuleID              *string               `json:"matchedRuleId"`
	FeedRevision               *string               `json:"feedRevision"`
	// VerdictSource is "live" or "fixture" on a semantic record, null on a deterministic one: a
	// fixture verdict is never semantic detection quality.
	VerdictSource *string         `json:"verdictSource"`
	Verdict       *VerdictSummary `json:"verdict"`
	AssessedAt    time.Time       `json:"assessedAt"`
}

// VerdictSummary is the decided semantic verdict schema, and only it: an extra stored key is
// never passed through.
type VerdictSummary struct {
	RiskCategory string  `json:"risk_category"`
	Score        float64 `json:"score"`
	ReasonCode   string  `json:"reason_code"`
}

// AssessmentPage is one window-cursor page of control assessments.
type AssessmentPage struct {
	Records    []AssessmentRecord `json:"records"`
	NextCursor string             `json:"nextCursor"`
}

// SecurityEventPage is one window-cursor page of the organization's sanitized events (X-12).
type SecurityEventPage struct {
	Events     []contracts.SafeEvent `json:"events"`
	NextCursor string                `json:"nextCursor"`
}

// SecuritySummary aggregates the organization's security records for the summary (X-93). Every
// count comes from stored records; timings are observed durations, never estimates.
type SecuritySummary struct {
	OrganizationID string            `json:"organizationId"`
	GeneratedAt    time.Time         `json:"generatedAt"`
	Runs           []StatusCount     `json:"runs"`
	Decisions      []DecisionCount   `json:"decisions"`
	Assessments    []AssessmentCount `json:"assessments"`
	ModelUsage     []PurposeUsage    `json:"modelUsage"`
	Timings        []PhaseTiming     `json:"timings"`
}

// StatusCount counts runs per X-11 status.
type StatusCount struct {
	Status contracts.RunStatus `json:"status"`
	Count  int64               `json:"count"`
}

// DecisionCount counts events per type, decision and reason, so the summary reconciles with
// the event records (X-104).
type DecisionCount struct {
	EventType  contracts.EventType      `json:"eventType"`
	Decision   *contracts.EventDecision `json:"decision"`
	ReasonCode *contracts.ReasonCode    `json:"reasonCode"`
	Count      int64                    `json:"count"`
}

// AssessmentCount counts control assessments per control, outcome and verdict source.
type AssessmentCount struct {
	ControlClass  string  `json:"controlClass"`
	ControlID     string  `json:"controlId"`
	Outcome       string  `json:"outcome"`
	VerdictSource *string `json:"verdictSource"`
	Count         int64   `json:"count"`
}

// PhaseTiming summarizes the observed spans of one measured phase, in microseconds.
type PhaseTiming struct {
	Phase              string `json:"phase"`
	Count              int64  `json:"count"`
	Failed             int64  `json:"failed"`
	MedianMicroseconds int64  `json:"medianMicroseconds"`
	P95Microseconds    int64  `json:"p95Microseconds"`
	MaxMicroseconds    int64  `json:"maxMicroseconds"`
}

// identifierPattern bounds the code-defined names this package passes on (control ids, outcomes,
// boundaries, phases, verdict categories).
var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

// Closed value sets of the assessment fields.
var (
	verdictSources = []string{"live", "fixture"}
	controlClasses = []string{"deterministic", "semantic"}
)

// maximumIdentifierLength is the X-12 schema bound on identifier fields.
const maximumIdentifierLength = 256

// SecuritySummaryHandler serves the security summary of the operator's organization.
func SecuritySummaryHandler(database Beginner) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		organizationID, ok := verifiedOrganization(responseWriter, request)
		if !ok {
			return
		}
		var summary SecuritySummary
		err := readTransaction(request.Context(), database, func(tx pgx.Tx) error {
			var readErr error
			summary, readErr = readSecuritySummary(request.Context(), tx, organizationID)
			return readErr
		})
		if err != nil {
			writeUnavailable(responseWriter, request)
			return
		}
		writeJSON(responseWriter, summary)
	})
}

// SecurityAssessmentsHandler serves the organization's control assessments by window cursor.
func SecurityAssessmentsHandler(database Beginner) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		organizationID, ok := verifiedOrganization(responseWriter, request)
		if !ok {
			return
		}
		cursor, limit, ok := windowPageQuery(responseWriter, request)
		if !ok {
			return
		}
		var page AssessmentPage
		err := readTransaction(request.Context(), database, func(tx pgx.Tx) error {
			var readErr error
			page, readErr = readAssessmentPage(request.Context(), tx, organizationID, cursor, limit)
			return readErr
		})
		if err != nil {
			writeUnavailable(responseWriter, request)
			return
		}
		writeJSON(responseWriter, page)
	})
}

// SecurityEventsHandler serves the organization's sanitized events, including those without a
// run (a rejected admission or reload), by window cursor.
func SecurityEventsHandler(database Beginner) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		organizationID, ok := verifiedOrganization(responseWriter, request)
		if !ok {
			return
		}
		cursor, limit, ok := windowPageQuery(responseWriter, request)
		if !ok {
			return
		}
		var page SecurityEventPage
		err := readTransaction(request.Context(), database, func(tx pgx.Tx) error {
			var readErr error
			page, readErr = readSecurityEventPage(request.Context(), tx, organizationID, cursor, limit)
			return readErr
		})
		if err != nil {
			writeUnavailable(responseWriter, request)
			return
		}
		writeJSON(responseWriter, page)
	})
}

func readAssessmentPage(ctx context.Context, tx pgx.Tx, organizationID string, cursor windowCursor, limit int) (AssessmentPage, error) {
	rows, err := tx.Query(ctx, windowHorizon+`
		SELECT horizon.high::text, record.id, record.run_id::text, record.evaluation_id::text,
			record.action_id::text, record.security_model_call_id::text, record.boundary,
			record.control_class, record.control_id, record.outcome, record.reason_code,
			record.admission_catalog_revision_id, record.evaluated_catalog_revision_id,
			record.matched_rule_id, record.feed_revision, record.verdict_source, record.verdict::text,
			record.assessed_at
		FROM runtime.control_assessments AS record, horizon
		WHERE record.organization_id = $1 AND `+windowPredicate+`
		ORDER BY record.id LIMIT $5`,
		organizationID, optionalXid8(cursor.Low), optionalXid8(cursor.High), cursor.AfterID, limit+1)
	if err != nil {
		return AssessmentPage{}, err
	}
	defer rows.Close()
	page := AssessmentPage{Records: []AssessmentRecord{}}
	var high uint64
	var lastID int64
	more := false
	for rows.Next() {
		if len(page.Records) == limit {
			more = true
			break
		}
		var highText string
		var recordID int64
		var reason, verdict *string
		var record AssessmentRecord
		if err := rows.Scan(&highText, &recordID, &record.RunID, &record.EvaluationID, &record.ActionID,
			&record.SecurityModelCallID, &record.Boundary, &record.ControlClass, &record.ControlID,
			&record.Outcome, &reason, &record.AdmissionCatalogRevisionID, &record.EvaluatedCatalogRevisionID,
			&record.MatchedRuleID, &record.FeedRevision, &record.VerdictSource, &verdict, &record.AssessedAt); err != nil {
			return AssessmentPage{}, err
		}
		if high, err = strconv.ParseUint(highText, 10, 64); err != nil {
			return AssessmentPage{}, errMalformedRecord
		}
		record.AssessmentID = strconv.FormatInt(recordID, 10)
		record.AssessedAt = record.AssessedAt.UTC()
		if record.ReasonCode, err = optionalReasonCode(reason); err != nil {
			return AssessmentPage{}, err
		}
		if verdict != nil {
			record.Verdict = &VerdictSummary{}
			if contracts.DecodeStrict([]byte(*verdict), record.Verdict) != nil {
				return AssessmentPage{}, errMalformedRecord
			}
		}
		if !validAssessment(record) {
			return AssessmentPage{}, errMalformedRecord
		}
		page.Records = append(page.Records, record)
		lastID = recordID
	}
	if err := rows.Err(); err != nil {
		return AssessmentPage{}, err
	}
	page.NextCursor = cursor.next(lastID, more, high).String()
	return page, nil
}

func readSecurityEventPage(ctx context.Context, tx pgx.Tx, organizationID string, cursor windowCursor, limit int) (SecurityEventPage, error) {
	rows, err := tx.Query(ctx, windowHorizon+`
		SELECT horizon.high::text, record.id, record.organization_id::text, record.run_id::text,
			record.action_id::text, record.event_type, record.decision, record.reason_code,
			record.catalog_revision_id, record.masked_summary::text, record.occurred_at
		FROM runtime.audit_events AS record, horizon
		WHERE record.organization_id = $1 AND `+windowPredicate+`
		ORDER BY record.id LIMIT $5`,
		organizationID, optionalXid8(cursor.Low), optionalXid8(cursor.High), cursor.AfterID, limit+1)
	if err != nil {
		return SecurityEventPage{}, err
	}
	defer rows.Close()
	page := SecurityEventPage{Events: []contracts.SafeEvent{}}
	var high uint64
	var lastID int64
	more := false
	for rows.Next() {
		if len(page.Events) == limit {
			more = true
			break
		}
		var highText, eventType, summary string
		var eventID int64
		var decision, reason *string
		var event contracts.SafeEvent
		if err := rows.Scan(&highText, &eventID, &event.OrganizationID, &event.RunID, &event.ActionID,
			&eventType, &decision, &reason, &event.CatalogRevisionID, &summary, &event.OccurredAt); err != nil {
			return SecurityEventPage{}, err
		}
		if high, err = strconv.ParseUint(highText, 10, 64); err != nil {
			return SecurityEventPage{}, errMalformedRecord
		}
		event.EventID = strconv.FormatInt(eventID, 10)
		event.EventType = contracts.EventType(eventType)
		event.OccurredAt = event.OccurredAt.UTC()
		if decision != nil {
			value := contracts.EventDecision(*decision)
			event.Decision = &value
		}
		if event.ReasonCode, err = optionalReasonCode(reason); err != nil {
			return SecurityEventPage{}, err
		}
		if contracts.DecodeStrict([]byte(summary), &event.MaskedSummary) != nil || !repository.ValidStoredEvent(event) {
			return SecurityEventPage{}, errMalformedRecord
		}
		page.Events = append(page.Events, event)
		lastID = eventID
	}
	if err := rows.Err(); err != nil {
		return SecurityEventPage{}, err
	}
	page.NextCursor = cursor.next(lastID, more, high).String()
	return page, nil
}

func readSecuritySummary(ctx context.Context, tx pgx.Tx, organizationID string) (SecuritySummary, error) {
	summary := SecuritySummary{
		OrganizationID: organizationID, Runs: []StatusCount{}, Decisions: []DecisionCount{},
		Assessments: []AssessmentCount{}, Timings: []PhaseTiming{},
	}
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&summary.GeneratedAt); err != nil {
		return SecuritySummary{}, err
	}
	summary.GeneratedAt = summary.GeneratedAt.UTC()

	err := collect(ctx, tx, `SELECT status, count(*) FROM runtime.runs WHERE organization_id = $1
		GROUP BY status ORDER BY status`, organizationID, func(rows pgx.Rows) error {
		var count StatusCount
		if err := rows.Scan(&count.Status, &count.Count); err != nil {
			return err
		}
		if !count.Status.Valid() {
			return errMalformedRecord
		}
		summary.Runs = append(summary.Runs, count)
		return nil
	})
	if err != nil {
		return SecuritySummary{}, err
	}

	err = collect(ctx, tx, `SELECT event_type, decision, reason_code, count(*) FROM runtime.audit_events
		WHERE organization_id = $1 GROUP BY event_type, decision, reason_code
		ORDER BY event_type, decision NULLS FIRST, reason_code NULLS FIRST`, organizationID, func(rows pgx.Rows) error {
		var count DecisionCount
		var decision, reason *string
		if err := rows.Scan(&count.EventType, &decision, &reason, &count.Count); err != nil {
			return err
		}
		if decision != nil {
			value := contracts.EventDecision(*decision)
			if !value.Valid() {
				return errMalformedRecord
			}
			count.Decision = &value
		}
		var err error
		if count.ReasonCode, err = optionalReasonCode(reason); err != nil {
			return err
		}
		if !count.EventType.Valid() {
			return errMalformedRecord
		}
		summary.Decisions = append(summary.Decisions, count)
		return nil
	})
	if err != nil {
		return SecuritySummary{}, err
	}

	err = collect(ctx, tx, `SELECT control_class, control_id, outcome, verdict_source, count(*)
		FROM runtime.control_assessments WHERE organization_id = $1
		GROUP BY control_class, control_id, outcome, verdict_source
		ORDER BY control_class, control_id, outcome, verdict_source NULLS FIRST`, organizationID, func(rows pgx.Rows) error {
		var count AssessmentCount
		if err := rows.Scan(&count.ControlClass, &count.ControlID, &count.Outcome, &count.VerdictSource, &count.Count); err != nil {
			return err
		}
		if !oneOf(count.ControlClass, controlClasses) || !identifierPattern.MatchString(count.ControlID) ||
			!identifierPattern.MatchString(count.Outcome) || !optionalOneOf(count.VerdictSource, verdictSources) {
			return errMalformedRecord
		}
		summary.Assessments = append(summary.Assessments, count)
		return nil
	})
	if err != nil {
		return SecuritySummary{}, err
	}

	if summary.ModelUsage, err = readPurposeUsage(ctx, tx, organizationID, nil); err != nil {
		return SecuritySummary{}, err
	}

	err = collect(ctx, tx, `SELECT phase, count(*), count(*) FILTER (WHERE failed),
			percentile_disc(0.5) WITHIN GROUP (ORDER BY duration_microseconds),
			percentile_disc(0.95) WITHIN GROUP (ORDER BY duration_microseconds),
			max(duration_microseconds)
		FROM runtime.timing_records WHERE organization_id = $1 GROUP BY phase ORDER BY phase`,
		organizationID, func(rows pgx.Rows) error {
			var timing PhaseTiming
			if err := rows.Scan(&timing.Phase, &timing.Count, &timing.Failed, &timing.MedianMicroseconds,
				&timing.P95Microseconds, &timing.MaxMicroseconds); err != nil {
				return err
			}
			if !identifierPattern.MatchString(timing.Phase) {
				return errMalformedRecord
			}
			summary.Timings = append(summary.Timings, timing)
			return nil
		})
	if err != nil {
		return SecuritySummary{}, err
	}
	return summary, nil
}

// collect runs an organization-scoped query and hands each row to scan.
func collect(ctx context.Context, tx pgx.Tx, query, organizationID string, scan func(pgx.Rows) error) error {
	rows, err := tx.Query(ctx, query, organizationID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func optionalReasonCode(value *string) (*contracts.ReasonCode, error) {
	if value == nil {
		return nil, nil
	}
	code := contracts.ReasonCode(*value)
	if !code.Valid() {
		return nil, errMalformedRecord
	}
	return &code, nil
}

func validAssessment(record AssessmentRecord) bool {
	semantic := record.ControlClass == "semantic"
	return uuidPattern.MatchString(record.RunID) && uuidPattern.MatchString(record.EvaluationID) &&
		optionalUUID(record.ActionID) && optionalUUID(record.SecurityModelCallID) &&
		identifierPattern.MatchString(record.Boundary) && oneOf(record.ControlClass, controlClasses) &&
		identifierPattern.MatchString(record.ControlID) && identifierPattern.MatchString(record.Outcome) &&
		record.AdmissionCatalogRevisionID > 0 && record.EvaluatedCatalogRevisionID > 0 &&
		optionalIdentifier(record.MatchedRuleID) && optionalIdentifier(record.FeedRevision) &&
		optionalOneOf(record.VerdictSource, verdictSources) && semantic == (record.VerdictSource != nil) &&
		(record.Verdict == nil || (semantic && validVerdict(*record.Verdict)))
}

func validVerdict(verdict VerdictSummary) bool {
	return identifierPattern.MatchString(verdict.RiskCategory) && identifierPattern.MatchString(verdict.ReasonCode) &&
		verdict.Score >= 0 && verdict.Score <= 1
}

func oneOf(value string, allowed []string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func optionalOneOf(value *string, allowed []string) bool {
	return value == nil || oneOf(*value, allowed)
}

// optionalIdentifier is the X-12 schema's identifier rule, as the repository applies it: non-empty,
// at most 256 characters, no control characters.
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

func optionalUUID(value *string) bool {
	return value == nil || uuidPattern.MatchString(*value)
}
