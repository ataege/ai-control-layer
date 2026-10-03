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
	"starter/services/gateway/internal/provenance"
)

// StartRunRoutePattern is the internal start-run command (X-28).
const StartRunRoutePattern = "POST /internal/runs"

// maximumStartRunBodyBytes bounds the start-run body; X-07 allows at most 100 invoice ids.
const maximumStartRunBodyBytes = 64 << 10

// RunAdmitter issues passports; *admission.Admitter implements it.
type RunAdmitter interface {
	Admit(ctx context.Context, operator contracts.OperatorContext, request contracts.StartRunRequest) (contracts.Passport, error)
}

// Dependencies are what the internal routes need.
type Dependencies struct {
	Admitter RunAdmitter
	// Database serves the stored report read (GO-37); the gateway pool in production.
	Database provenance.Beginner
}

// Commands returns every internal product route for httpserver.Options.InternalCommands.
func Commands(dependencies Dependencies) []httpserver.InternalCommand {
	return []httpserver.InternalCommand{
		{Pattern: StartRunRoutePattern, Handler: StartRunHandler(dependencies.Admitter)},
		{Pattern: provenance.StoredReportRoutePattern, Handler: provenance.StoredReportHandler(dependencies.Database, StoredReportViewer)},
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

// StoredReportViewer derives the GO-37 viewer from the verified operator only. Every verified
// operator of the organization may read its internal reports (lead decision).
func StoredReportViewer(request *http.Request) (provenance.Viewer, bool) {
	operator, verified := operatorcontext.FromContext(request.Context())
	if !verified {
		return provenance.Viewer{}, false
	}
	return provenance.Viewer{OrganizationID: operator.OrganizationID, MayReadInternal: true}, true
}
