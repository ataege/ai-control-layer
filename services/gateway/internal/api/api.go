// Package api holds the gateway's internal product routes. Every route is registered through
// httpserver.Options.InternalCommands, so it runs only after the service token and the verified
// operator context (GO-21); handlers take identity from operatorcontext.FromContext alone.
package api

import (
	"context"
	"errors"
	"net/http"

	"starter/services/gateway/internal/admission"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/health"
	"starter/services/gateway/internal/httpserver"
	"starter/services/gateway/internal/operatorcontext"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/provenance"
	"starter/services/gateway/internal/repository"
)

// StartRunRoutePattern is the internal start-run command (X-28).
const StartRunRoutePattern = "POST /internal/runs"

// CancelRunRoutePattern is the internal cancel command (X-42, GO-41).
const CancelRunRoutePattern = "POST /internal/runs/{runId}/cancel"

// maximumCancelBodyBytes bounds the cancel body, which is empty or {}.
const maximumCancelBodyBytes = 1 << 10

// maximumStartRunBodyBytes bounds the start-run body; X-07 allows at most 100 invoice ids.
const maximumStartRunBodyBytes = 64 << 10

// RunAdmitter issues passports; *admission.Admitter implements it.
type RunAdmitter interface {
	Admit(ctx context.Context, operator contracts.OperatorContext, request contracts.StartRunRequest) (contracts.Passport, error)
}

// RunCanceller records cancellations; *repository.Repository implements it.
type RunCanceller interface {
	CancelRun(ctx context.Context, organizationID, runID string) (contracts.RunState, error)
}

// Approvals decides approvals and serves frozen review payloads (lane w3's GO-44);
// *policy.Approvals implements it.
type Approvals interface {
	policy.ApprovalDecider
	policy.ReviewReader
}

// Dependencies are what the internal routes need.
type Dependencies struct {
	Admitter  RunAdmitter
	Canceller RunCanceller
	Approvals Approvals
	// Database serves the stored report read (GO-37); the gateway pool in production.
	Database provenance.Beginner
}

// Commands returns every internal product route for httpserver.Options.InternalCommands.
func Commands(dependencies Dependencies) []httpserver.InternalCommand {
	return []httpserver.InternalCommand{
		{Pattern: StartRunRoutePattern, Handler: StartRunHandler(dependencies.Admitter)},
		{Pattern: CancelRunRoutePattern, Handler: CancelRunHandler(dependencies.Canceller)},
		{Pattern: provenance.StoredReportRoutePattern, Handler: provenance.StoredReportHandler(dependencies.Database, StoredReportViewer)},
		{Pattern: policy.ApprovalRoutePattern, Handler: policy.ApprovalHandler(dependencies.Approvals)},
		{Pattern: policy.ReviewRoutePattern, Handler: policy.ReviewHandler(dependencies.Approvals)},
	}
}

// StartRunHandler admits a start-run command. It answers 201 with the run and passport ids, 400
// with the X-13 reason code and the scope or limit to change, or 503 decision_unavailable when
// admission could not decide. It never waits on a model or tool request: the run proceeds in
// the worker.
func StartRunHandler(admitter RunAdmitter) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		operator, verified := operatorcontext.FromContext(request.Context())
		if !verified || admitter == nil {
			// The route guard always sets the operator; its absence is never an allow.
			httpserver.WriteError(responseWriter, request, http.StatusUnauthorized, "unauthorized", "Missing or invalid operator context.")
			return
		}
		var startRequest contracts.StartRunRequest
		if !httpserver.DecodeJSONBody(responseWriter, request, maximumStartRunBodyBytes, &startRequest) {
			return
		}
		passport, err := admitter.Admit(request.Context(), operator, startRequest)
		var rejection *admission.Rejection
		switch {
		case errors.As(err, &rejection):
			httpserver.WriteError(responseWriter, request, http.StatusBadRequest, string(rejection.Code), rejection.Message)
		case err != nil:
			httpserver.WriteError(responseWriter, request, http.StatusServiceUnavailable, string(contracts.ReasonDecisionUnavailable),
				"Admission could not be decided; no run was started.")
		default:
			health.WriteJSON(responseWriter, http.StatusCreated, contracts.StartRunResponse{RunID: passport.RunID, PassportID: passport.PassportID})
		}
	})
}

// CancelRunHandler records a cancellation of a run of the operator's organization and answers
// 200 with its X-11 state. Any verified operator of the organization may cancel its runs (the
// organization is the authority checked today). Another organization's or an unknown run is
// 404; the body must be empty or {}. Committed effects are not reversed.
func CancelRunHandler(canceller RunCanceller) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		operator, verified := operatorcontext.FromContext(request.Context())
		if !verified || canceller == nil {
			httpserver.WriteError(responseWriter, request, http.StatusUnauthorized, "unauthorized", "Missing or invalid operator context.")
			return
		}
		if request.ContentLength != 0 {
			var emptyCommand struct{}
			if !httpserver.DecodeJSONBody(responseWriter, request, maximumCancelBodyBytes, &emptyCommand) {
				return
			}
		}
		state, err := canceller.CancelRun(request.Context(), operator.OrganizationID, request.PathValue("runId"))
		switch {
		case errors.Is(err, repository.ErrNotFound), errors.Is(err, repository.ErrInvalid):
			// An unknown id and another organization's run look the same.
			httpserver.WriteError(responseWriter, request, http.StatusNotFound, "not_found", "Run not found.")
		case err != nil:
			httpserver.WriteError(responseWriter, request, http.StatusServiceUnavailable, string(contracts.ReasonDecisionUnavailable),
				"The cancellation could not be recorded.")
		default:
			health.WriteJSON(responseWriter, http.StatusOK, state)
		}
	})
}

// StoredReportViewer derives the GO-37 viewer from the verified operator only. Every verified
// operator of the organization may read its internal reports (lead decision).
func StoredReportViewer(request *http.Request) (provenance.Viewer, bool) {
	operator, verified := operatorcontext.FromContext(request.Context())
	if !verified {
		return provenance.Viewer{}, false
	}
	return provenance.Viewer{OrganizationID: operator.OrganizationID, MayReadInternal: true}, true
}
