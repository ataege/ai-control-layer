package policy

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/contracts"
)

// DenialFeedback is the structured reason the model receives after a denial (GO-29). It holds a
// stable code, a fixed safe message and, where one exists, a permitted alternative, never a
// protected value, review content or the raw arguments. The alternative grants no authority: the
// next proposal is evaluated by the gate like any other.
type DenialFeedback struct {
	ReasonCode          ReasonCode `json:"reason_code"`
	SafeMessage         string     `json:"safe_message"`
	AlternativeTemplate string     `json:"alternative_template,omitempty"`
	// PermittedInvoiceIDs are the passport's own invoice references, offered after an
	// out-of-scope read so the model can return to the permitted record set.
	PermittedInvoiceIDs []string `json:"permitted_invoice_ids,omitempty"`
}

// safeMessages are fixed feedback texts written for the model's correction. A reason without one
// takes the X-13 safe message of internal/contracts, the single message table, and only a value
// outside the vocabulary gets the generic text.
var safeMessages = map[ReasonCode]string{
	ReasonResourceOutOfScope:     "The requested record is outside this task's scope.",
	ReasonDestinationNotAllowed:  "The destination is not a registered recipient of this task.",
	ReasonReportExportRestricted: "This report inherits an Internal only restriction and cannot be sent to the vendor.",
	ReasonReportLineageMissing:   "This report has no verifiable source trail and cannot be sent.",
	ReasonTemplateNotAllowed:     "This report template is not permitted for this task.",
	ReasonToolNotRegistered:      "That tool is not available.",
	ReasonToolNotAllowed:         "That tool is not permitted for this task.",
	ReasonInvalidArguments:       "The tool arguments are not valid for that tool.",
	ReasonRunExpired:             "The task has expired.",
	ReasonDecisionUnavailable:    "The action could not be checked and was not run.",
	ReasonAllowanceExhausted:     "The task has no remaining allowance for that.",
	ReasonActionChanged:          "The action changed after it was checked and was not run.",
}

const genericSafeMessage = "The action was denied by the task's controls."

// BuildDenialFeedback turns a deny into feedback for the model. An export denial names the vendor
// report template only when the passport permits that template and has invoice sources for it
// ("offered only if the existing task grant permits the template and source set").
func BuildDenialFeedback(decision Decision, scope PassportScope) DenialFeedback {
	feedback := DenialFeedback{ReasonCode: decision.ReasonCode, SafeMessage: genericSafeMessage}
	if message, known := safeMessages[decision.ReasonCode]; known {
		feedback.SafeMessage = message
	} else if contractCode := contracts.ReasonCode(decision.ReasonCode); contractCode.Valid() {
		feedback.SafeMessage = contractCode.SafeMessage()
	}
	if decision.AlternativeTemplate != "" &&
		containsString(scope.AllowedTemplates, decision.AlternativeTemplate) &&
		containsTool(scope.AllowedTools, ToolCreateReport) &&
		len(scope.AllowedInvoiceIDs) > 0 {
		feedback.AlternativeTemplate = decision.AlternativeTemplate
	}
	if decision.ReasonCode == ReasonResourceOutOfScope && len(scope.AllowedInvoiceIDs) > 0 {
		feedback.PermittedInvoiceIDs = append([]string(nil), scope.AllowedInvoiceIDs...)
	}
	return feedback
}

// CorrectionVerdict says whether the run may continue after a denial.
type CorrectionVerdict struct {
	Continue        bool
	CorrectionsUsed int
	Limit           int
	// StopReason is set when Continue is false.
	StopReason ReasonCode
}

// CheckCorrections applies the passport's correction limit to the number of denials so far
// (this one included): the run continues while used <= limit, so a limit of 2 allows two
// corrections and stops at the third denial.
func CheckCorrections(correctionsUsed, correctionLimit int) CorrectionVerdict {
	verdict := CorrectionVerdict{CorrectionsUsed: correctionsUsed, Limit: correctionLimit}
	if correctionLimit < 0 || correctionsUsed > correctionLimit {
		verdict.StopReason = ReasonAllowanceExhausted
		return verdict
	}
	verdict.Continue = true
	return verdict
}

// ErrCorrectionsUnavailable means the run's denials could not be counted; the caller stops.
var ErrCorrectionsUnavailable = errors.New("correction count unavailable")

// CorrectionCounter counts a run's denials from its durable decision events, so the count
// survives a worker restart and includes denials that have no action row.
type CorrectionCounter struct{ pool *pgxpool.Pool }

// NewCorrectionCounter returns a counter on the given pool. It creates nothing.
func NewCorrectionCounter(pool *pgxpool.Pool) *CorrectionCounter {
	return &CorrectionCounter{pool: pool}
}

// CorrectionsUsed returns how many proposals of the run the gate has denied.
func (counter *CorrectionCounter) CorrectionsUsed(ctx context.Context, run RunIdentity) (int, error) {
	if counter.pool == nil {
		return 0, ErrCorrectionsUnavailable
	}
	var denials int
	err := counter.pool.QueryRow(ctx,
		`SELECT count(*) FROM runtime.audit_events
		  WHERE organization_id = $1 AND run_id = $2 AND event_type = ANY($3)`,
		run.OrganizationID, run.RunID,
		[]string{string(contracts.EventActionDenied), string(contracts.EventReportExportDenied)}).Scan(&denials)
	if err != nil {
		return 0, ErrCorrectionsUnavailable
	}
	return denials, nil
}
