package contracts

// reasonMessages are the X-13 safe operator messages: fixed text per reason code, never built from
// request values, record content, model output or error text, so they may appear in events, error
// envelopes and the interface as they are (GO-58). They name no run status: one code can end a
// run in different states, and the event's status says which.
var reasonMessages = map[ReasonCode]string{
	ReasonResourceOutOfScope:           "The requested record is outside this task's scope.",
	ReasonDestinationNotAllowed:        "The destination is not a registered recipient of this task.",
	ReasonReportExportRestricted:       "This report inherits an Internal only restriction and cannot be sent to the vendor.",
	ReasonReportLineageMissing:         "This report has no verifiable source trail and cannot be sent.",
	ReasonSourcePolicyChanged:          "A source policy changed after the check, so the action was not run.",
	ReasonTemplateNotAllowed:           "This report template is not permitted for this task.",
	ReasonApprovalRequired:             "This action waits for a reviewer's approval of the exact action.",
	ReasonApprovalExpired:              "The approval expired before the action ran; a new review is needed.",
	ReasonActionChanged:                "The action changed after it was checked and was not run.",
	ReasonResourceVersionChanged:       "A source record changed after the check, so the action was not run.",
	ReasonAllowanceExhausted:           "The run reached one of its limits (model calls, tokens, time or corrections), so no further work of that kind is dispatched.",
	ReasonRunCancelled:                 "The run was cancelled by an operator.",
	ReasonOutcomeUnknown:               "Whether the last request took effect is unknown, so it needs operator attention before the run can continue.",
	ReasonSemanticInjectionDetected:    "The security check found instruction-like content and withheld it.",
	ReasonSecurityEvaluatorUnavailable: "The security check could not run, so the content or action it guards did not continue.",
	ReasonSecurityAllowanceExhausted:   "The security check has no remaining allowance, so nothing further is checked or dispatched.",
	ReasonContentRedacted:              "Part of the content matched a protected pattern and was masked.",
	ReasonSignatureMatch:               "The content matched a known attack signature and was withheld.",
	ReasonPolicyReloadRejected:         "The new policy was rejected; the previous active policy stays in force.",
	ReasonModelNotAllowed:              "The configured model is not allowed by the active policy, so no model request is sent.",
	ReasonMultipleActionsNotSupported:  "The model proposed several actions at once; only one action per step is allowed.",
	ReasonRunExpired:                   "The task has expired.",
	ReasonToolNotRegistered:            "That tool is not available.",
	ReasonInvalidArguments:             "The request or tool arguments are not valid.",
	ReasonToolNotAllowed:               "That tool is not permitted for this task.",
	ReasonDecisionUnavailable:          "A required check could not be made, so nothing was run.",
	ReasonContentBlocked:               "The content was withheld by a security control.",
	ReasonContentTooLarge:              "The content exceeds the inspected size limit and was withheld.",
	ReasonLimitNotAllowed:              "The requested limit exceeds what the active policy allows.",
}

// genericReasonMessage answers a value outside the vocabulary; it names nothing about the value.
const genericReasonMessage = "The request was stopped by the task's controls."

// SafeMessage returns the code's X-13 safe operator message.
func (code ReasonCode) SafeMessage() string {
	if message, known := reasonMessages[code]; known {
		return message
	}
	return genericReasonMessage
}
