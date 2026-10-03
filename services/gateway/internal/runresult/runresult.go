// Package runresult validates the narrow final result of a run (GO-26): "a structured status with
// authorized report references". The model's final answer is exactly one JSON object
// {"status":"completed","report_ids":[...]} naming one or two reports this run created; anything
// else is rejected, never trimmed. Only the validated reference is persisted, never model prose:
// the interface renders the substantive content from the stored reports.
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

// Parse checks the answer's format alone: one JSON object with exactly status and report_ids, the
// completed status, one or two unique lowercase report uuids, no text around it.
func Parse(answer string) (FinalResult, contracts.ReasonCode) {
	// The bound applies to the whole answer, before surrounding whitespace is ignored.
	if len(answer) > maximumAnswerBytes {
		return FinalResult{}, contracts.ReasonInvalidArguments
	}
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" || !utf8.ValidString(trimmed) || !uniqueKeys(trimmed) {
		return FinalResult{}, contracts.ReasonInvalidArguments
	}
	var document struct {
		Status    *string  `json:"status"`
		ReportIDs []string `json:"report_ids"`
	}
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return FinalResult{}, contracts.ReasonInvalidArguments
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return FinalResult{}, contracts.ReasonInvalidArguments
	}
	if document.Status == nil || *document.Status != CompletedStatus ||
		len(document.ReportIDs) == 0 || len(document.ReportIDs) > MaximumReports {
		return FinalResult{}, contracts.ReasonInvalidArguments
	}
	seen := make(map[string]bool, len(document.ReportIDs))
	for _, reportID := range document.ReportIDs {
		if !uuidPattern.MatchString(reportID) || seen[reportID] {
			return FinalResult{}, contracts.ReasonInvalidArguments
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
