package policy

import (
	"context"
	"errors"
	"net/http"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/health"
	"starter/services/gateway/internal/httpserver"
	"starter/services/gateway/internal/operatorcontext"
)

// Internal routes of the approval flow (GO-44). 3c mounts them through
// httpserver.Options.InternalCommands, so they run only behind the service token and the
// verified operator context.
const (
	ApprovalRoutePattern = "POST /internal/actions/{actionId}/approval"
	ReviewRoutePattern   = "GET /internal/actions/{actionId}/review"
)

// maximumApprovalBodyBytes bounds the approval body, which holds one short field.
const maximumApprovalBodyBytes = 1 << 10

// ApprovalDecider is what the approval handler needs; *Approvals implements it.
type ApprovalDecider interface {
	Decide(ctx context.Context, operator contracts.OperatorContext, actionID string, choice ApprovalChoice) (ApprovalResult, error)
}

// ReviewReader is what the review handler needs; *Approvals implements it.
type ReviewReader interface {
	FrozenReviewFor(ctx context.Context, operator contracts.OperatorContext, actionID string) (ReviewPayload, error)
}

// approvalResponse is the answer to a decided approval: references only.
type approvalResponse struct {
	ApprovalID string         `json:"approvalId"`
	ActionID   string         `json:"actionId"`
	RunID      string         `json:"runId"`
	Decision   ApprovalChoice `json:"decision"`
}

// ApprovalHandler decides an approval for the stored action in the path. Identity comes from the
// verified operator context only.
func ApprovalHandler(decider ApprovalDecider) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		operator, verified := operatorcontext.FromContext(request.Context())
		if !verified || decider == nil {
			httpserver.WriteError(responseWriter, request, http.StatusUnauthorized, "unauthorized", "Missing or invalid operator context.")
			return
		}
		// X-10: the decision only. The strict decoder rejects unknown fields, so a body carrying any
		// payload stores no grant; a missing or unknown decision is rejected before deciding.
		var body contracts.ApprovalDecision
		if !httpserver.DecodeJSONBody(responseWriter, request, maximumApprovalBodyBytes, &body) {
			return
		}
		if !body.Decision.Valid() {
			httpserver.WriteError(responseWriter, request, http.StatusBadRequest, "bad_request", "The request body is not a valid command.")
			return
		}
		result, err := decider.Decide(request.Context(), operator, request.PathValue("actionId"), body.Decision)
		if err != nil {
			writeApprovalError(responseWriter, request, err)
			return
		}
		health.WriteJSON(responseWriter, http.StatusOK, approvalResponse{
			ApprovalID: result.ApprovalID, ActionID: result.ActionID, RunID: result.RunID, Decision: result.Decision,
		})
	})
}

// ReviewHandler returns the frozen review material of the action in the path to an authorized
// reviewer of its organization. It is the only place the exact content and recipient leave Go.
func ReviewHandler(reader ReviewReader) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		operator, verified := operatorcontext.FromContext(request.Context())
		if !verified || reader == nil {
			httpserver.WriteError(responseWriter, request, http.StatusUnauthorized, "unauthorized", "Missing or invalid operator context.")
			return
		}
		payload, err := reader.FrozenReviewFor(request.Context(), operator, request.PathValue("actionId"))
		if err != nil {
			writeApprovalError(responseWriter, request, err)
			return
		}
		health.WriteJSON(responseWriter, http.StatusOK, payload)
	})
}

// writeApprovalError maps approval failures to stable answers. An action of another organization
// is answered like a missing one.
func writeApprovalError(responseWriter http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, ErrApprovalInvalid):
		httpserver.WriteError(responseWriter, request, http.StatusBadRequest, "bad_request", "The request is not a valid approval command.")
	case errors.Is(err, ErrNotReviewer):
		httpserver.WriteError(responseWriter, request, http.StatusForbidden, "forbidden", "The operator is not a reviewer of this organization.")
	case errors.Is(err, ErrApprovalNotFound):
		httpserver.WriteError(responseWriter, request, http.StatusNotFound, "not_found", "No action awaits approval here.")
	case errors.Is(err, ErrApprovalExpired):
		httpserver.WriteError(responseWriter, request, http.StatusConflict, string(contracts.ReasonApprovalExpired), "The approval has expired.")
	case errors.Is(err, ErrApprovalChanged):
		httpserver.WriteError(responseWriter, request, http.StatusConflict, string(contracts.ReasonActionChanged), "The action changed after it was frozen for review.")
	case errors.Is(err, ErrApprovalClosed):
		httpserver.WriteError(responseWriter, request, http.StatusConflict, "conflict", "The approval is already decided.")
	default:
		httpserver.WriteError(responseWriter, request, http.StatusServiceUnavailable, string(contracts.ReasonDecisionUnavailable), "The approval could not be decided; nothing was stored.")
	}
}
