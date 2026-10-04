// Package runresult validates the narrow final result of a run (GO-26): "a structured status with
// authorized report references". The model's final answer must contain exactly one top-level JSON
// object {"status":"completed","report_ids":[...]} naming one or two reports this run created; the
// object itself is checked strictly. Text around it, a code fence or whitespace is format leniency
// only: it is ignored and never stored. Two objects, or none, are rejected. Only the validated
// reference is persisted, never model prose: the interface renders the substantive content from the
// stored reports.
package runresult

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
)

// ErrUnavailable means the reports could not be read; the run must not complete on it.
var ErrUnavailable = errors.New("final result validation unavailable")

const (
	// CompletedStatus is the only supported final status.
	CompletedStatus = "completed"
	// MaximumReports bounds the references: the internal and the vendor report.
	MaximumReports = 2
	// maximumAnswerBytes bounds parsing work on model output.
	maximumAnswerBytes = 4 << 10
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// FinalResult is the validated final result; its JSON form is the persisted result reference.
type FinalResult struct {
	ReportIDs []string `json:"report_ids"`
}

// Querier reads reports; a pool or a transaction.
type Querier interface {
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
}

// Fixed causes of a rejected final answer: the X-13 rejectionCause values, for the log and the
// event's safe fields only, never the answer's text.
const (
	// CauseNotJSON: empty, over the size cap, invalid UTF-8, or no top-level JSON object.
	CauseNotJSON = "not_json"
	// CauseExtraText: more than one top-level JSON object in the answer (text around one object is
	// accepted).
	CauseExtraText = "extra_text"
	// CauseCodeFence is a contract value that Parse no longer returns: a code fence around the object
	// is format leniency now, like any other text around it.
	CauseCodeFence = "code_fence"
	// CauseWrongStatus: status missing or not "completed".
	CauseWrongStatus = "wrong_status"
	// CauseWrongFields: an unknown or repeated key, no or more than two report ids, an id that is
	// not a lowercase uuid, or a repeated id.
	CauseWrongFields = "wrong_fields"
	// CauseUnknownReport is for the caller: Validate's resource_out_of_scope (a report this run
	// did not create). Cause checks the format only and never returns it.
	CauseUnknownReport = "unknown_report"
)

// Parse checks the answer's format alone: exactly one top-level JSON object with exactly status and
// report_ids, the completed status, one or two unique lowercase report uuids. Text, a code fence or
// whitespace around the object is ignored; two objects, or none, are rejected.
func Parse(answer string) (FinalResult, contracts.ReasonCode) {
	result, cause := parse(answer)
	if cause != "" {
		return FinalResult{}, contracts.ReasonInvalidArguments
	}
	return result, ""
}

// Cause names why Parse rejects the answer, from the fixed Cause* vocabulary, or "" when the
// format is valid. It returns no part of the answer, so the caller may log it.
func Cause(answer string) string {
	_, cause := parse(answer)
	return cause
}

// parse is the single format check behind Parse and Cause.
func parse(answer string) (FinalResult, string) {
	// The bound applies to the whole answer, before anything around the object is ignored.
	if len(answer) > maximumAnswerBytes || !utf8.ValidString(answer) {
		return FinalResult{}, CauseNotJSON
	}
	objects := topLevelObjects(answer)
	switch len(objects) {
	case 0:
		return FinalResult{}, CauseNotJSON
	case 1:
		return parseObject(objects[0])
	default:
		return FinalResult{}, CauseExtraText
	}
}

// topLevelObjects returns every JSON object in the answer that is not inside another JSON value,
// in order. Anything that is not a complete JSON value (prose, a code fence, a lone brace) is
// skipped, and a complete JSON array is skipped whole, so an object inside an array is not a
// top-level object.
func topLevelObjects(answer string) []string {
	var objects []string
	for position := 0; position < len(answer); {
		if answer[position] != '{' && answer[position] != '[' {
			position++
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(answer[position:]))
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			position++
			continue
		}
		end := position + int(decoder.InputOffset())
		if answer[position] == '{' {
			objects = append(objects, answer[position:end])
		}
		position = end
	}
	return objects
}

// parseObject is the strict check of the one JSON object: only status and report_ids, no repeated
// key, the completed status and one or two unique lowercase report uuids.
func parseObject(object string) (FinalResult, string) {
	var document struct {
		Status    *string  `json:"status"`
		ReportIDs []string `json:"report_ids"`
	}
	decoder := json.NewDecoder(strings.NewReader(object))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		// encoding/json names a rejected field this way; every other decode error is syntax or type.
		if strings.Contains(err.Error(), "unknown field") {
			return FinalResult{}, CauseWrongFields
		}
		return FinalResult{}, CauseNotJSON
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return FinalResult{}, CauseExtraText
	}
	if !uniqueKeys(object) {
		return FinalResult{}, CauseWrongFields
	}
	if document.Status == nil || *document.Status != CompletedStatus {
		return FinalResult{}, CauseWrongStatus
	}
	if len(document.ReportIDs) == 0 || len(document.ReportIDs) > MaximumReports {
		return FinalResult{}, CauseWrongFields
	}
	seen := make(map[string]bool, len(document.ReportIDs))
	for _, reportID := range document.ReportIDs {
		if !uuidPattern.MatchString(reportID) || seen[reportID] {
			return FinalResult{}, CauseWrongFields
		}
		seen[reportID] = true
	}
	return FinalResult{ReportIDs: document.ReportIDs}, ""
}

// Validate parses the answer and checks every named report is a report this run created in the
// organization. It returns the canonical result reference to persist with the completion, or a
// rejection reason, or ErrUnavailable when the reports could not be read.
func Validate(ctx context.Context, querier Querier, organizationID, runID, answer string) (string, contracts.ReasonCode, error) {
	if ctx == nil || querier == nil || !uuidPattern.MatchString(organizationID) || !uuidPattern.MatchString(runID) {
		return "", "", ErrUnavailable
	}
	result, reason := Parse(answer)
	if reason != "" {
		return "", reason, nil
	}
	rows, err := querier.Query(ctx, `SELECT id::text FROM demo.reports
		WHERE organization_id = $1 AND run_id = $2 AND id = ANY($3::uuid[])`, organizationID, runID, result.ReportIDs)
	if err != nil {
		return "", "", ErrUnavailable
	}
	found := make(map[string]bool, len(result.ReportIDs))
	for rows.Next() {
		var reportID string
		if err := rows.Scan(&reportID); err != nil {
			rows.Close()
			return "", "", ErrUnavailable
		}
		found[reportID] = true
	}
	rows.Close()
	if rows.Err() != nil {
		return "", "", ErrUnavailable
	}
	// Another run's or organization's report looks the same as an unknown one.
	for _, reportID := range result.ReportIDs {
		if !found[reportID] {
			return "", contracts.ReasonResourceOutOfScope, nil
		}
	}
	reference, err := json.Marshal(result)
	if err != nil {
		return "", "", ErrUnavailable
	}
	return string(reference), "", nil
}

// uniqueKeys rejects an object that repeats a key at its top level (encoding/json keeps the last).
func uniqueKeys(document string) bool {
	decoder := json.NewDecoder(bytes.NewReader([]byte(document)))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		key, isKey := token.(string)
		if err != nil || !isKey || seen[key] {
			return false
		}
		seen[key] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return false
		}
	}
	return true
}

// FinalAnswerInstruction is the one wording of the final format for the agent's instructions
// (lane f3), so the prompt and this validator cannot drift.
const FinalAnswerInstruction = `When the task is finished, reply with only this JSON object and nothing else: ` +
	`{"status":"completed","report_ids":["<report id>"]}, naming one or two reports you created in this run by the ids create_report returned.`
