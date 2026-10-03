package tools

import (
	"errors"
	"slices"
)

// FailureClass classifies how one RunEffect call ended (GO-53, Figure 9).
type FailureClass string

const (
	// FailureNone: the call returned a result (succeeded, or the adapter's own refusal).
	FailureNone FailureClass = "none"
	// FailureKnownNoEffect: RunEffect returned a storage error and the executor rolled the
	// transaction back. Every demo effect is a local write in that one transaction (GO-34), so a
	// rollback is proof that nothing was committed: the outcome is a known failure.
	FailureKnownNoEffect FailureClass = "known_no_effect"
	// FailurePrecondition: the request no longer matches the stored action, attempt or passport.
	// Retrying cannot help; the executor pauses or fails the run.
	FailurePrecondition FailureClass = "precondition"
)

// ClassifyRunError classifies the error RunEffect returned, after the executor rolled back.
// A failed commit is not classified here: the executor cannot know whether it reached the
// database, so that outcome is unknown and is never retried.
func ClassifyRunError(err error) FailureClass {
	switch {
	case err == nil:
		return FailureNone
	case errors.Is(err, errPrecondition):
		return FailurePrecondition
	default:
		return FailureKnownNoEffect
	}
}

// retrySafeTools are the tools whose known no-effect failure may be retried under the same
// action, idempotency key and consumed grant, with a new attempt and fresh checks (GO-07): the
// reads write nothing, and create_report and queue_report are unique per action in the database,
// so a retry can never create a second report or outbox row.
var retrySafeTools = []string{ToolReadInvoice, ToolReadVendor, ToolCreateReport, ToolQueueReport}

// RetrySafe reports whether a failure of this class for this tool may be retried under the same
// action. Only known no-effect failures of registered tools qualify; an adapter refusal
// (Outcome "failed") is deterministic and never retried, and an unknown outcome never is.
func RetrySafe(tool string, class FailureClass) bool {
	return class == FailureKnownNoEffect && slices.Contains(retrySafeTools, tool)
}
