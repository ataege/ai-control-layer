package reads

import (
	"context"
	"net/http"

	"starter/services/gateway/internal/contracts"
)

// PassportRoutePattern is the route the api package mounts for the run's Task Passport (WEB-08).
const PassportRoutePattern = "GET /internal/runs/{runId}/passport"

// PassportReader reads the passport of one run of an organization; *repository.Repository
// implements it.
type PassportReader interface {
	Passport(ctx context.Context, organizationID, runID string) (contracts.Passport, error)
}

// PassportHandler serves the X-08 passport of one run of the operator's organization, as stored
// and strictly decoded. A run of another organization or an unknown run is 404.
func PassportHandler(reader PassportReader) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		organizationID, runID, ok := runRequest(responseWriter, request)
		if !ok {
			return
		}
		if reader == nil {
			writeUnavailable(responseWriter, request)
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), readTimeout)
		defer cancel()
		passport, err := reader.Passport(ctx, organizationID, runID)
		if err != nil {
			writeReadError(responseWriter, request, err)
			return
		}
		writeJSON(responseWriter, passport)
	})
}
