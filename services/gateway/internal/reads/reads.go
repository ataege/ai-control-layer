// Package reads serves the operator reads of the runtime records as private Go endpoints: the run
// state, usage and events (GO-24, X-29 and X-30) and the security decision records (GO-83). The
// `read path` decision chose private Go /internal endpoints. The api package mounts every handler
// through httpserver.Options.InternalCommands, so each runs after the service token and the
// verified operator context; the organization comes from operatorcontext.FromContext alone, and a
// run id in the path is a reference, never authority.
package reads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/httpserver"
	"starter/services/gateway/internal/operatorcontext"
	"starter/services/gateway/internal/repository"
)

// Route patterns the api package mounts.
const (
	RunStateRoutePattern            = "GET /internal/runs/{runId}"
	RunUsageRoutePattern            = "GET /internal/runs/{runId}/usage"
	RunEventsRoutePattern           = "GET /internal/runs/{runId}/events"
	SecuritySummaryRoutePattern     = "GET /internal/security/summary"
	SecurityAssessmentsRoutePattern = "GET /internal/security/assessments"
	SecurityEventsRoutePattern      = "GET /internal/security/events"
)

// Page sizes: a read answers well inside the server's write timeout (polling, no stream).
const (
	defaultPageSize = 100
	maximumPageSize = 500
	readTimeout     = 5 * time.Second
)

// RunStateReader reads one run of an organization; *repository.Repository implements it.
type RunStateReader interface {
	RunState(ctx context.Context, organizationID, runID string) (contracts.RunState, error)
}

// RunEventsReader reads a cursor page of one run's events; *repository.Repository implements it.
type RunEventsReader interface {
	RunEvents(ctx context.Context, organizationID, runID string, afterEventID int64, limit int) ([]contracts.SafeEvent, error)
}

// Beginner starts a transaction: the gateway pool, or an enclosing transaction in tests.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// RunEventPage is one cursor page of a run's events (X-30). NextCursor is the id to pass as
// `after` next time; it equals the request's cursor when no new event was committed.
type RunEventPage struct {
	Events     []contracts.SafeEvent `json:"events"`
	NextCursor string                `json:"nextCursor"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// errMalformedRecord means a stored row is outside its contract; it is never served.
var errMalformedRecord = errors.New("reads: stored record outside its contract")

// RunStateHandler serves the X-11 state of one run of the operator's organization.
func RunStateHandler(reader RunStateReader) http.Handler {
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
		state, err := reader.RunState(ctx, organizationID, runID)
		if err != nil {
			writeReadError(responseWriter, request, err)
			return
		}
		writeJSON(responseWriter, state)
	})
}

// RunEventsHandler serves the run's sanitized events after the `after` cursor, in order. A run's
// events commit in id order (repository.AppendEvent locks the run), so paging by the last id
// never skips or repeats one.
func RunEventsHandler(reader RunEventsReader) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		organizationID, runID, ok := runRequest(responseWriter, request)
		if !ok {
			return
		}
		afterEventID, limit, ok := eventPageQuery(responseWriter, request)
		if !ok {
			return
		}
		if reader == nil {
			writeUnavailable(responseWriter, request)
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), readTimeout)
		defer cancel()
		events, err := reader.RunEvents(ctx, organizationID, runID, afterEventID, limit)
		if err != nil {
			writeReadError(responseWriter, request, err)
			return
		}
		page := RunEventPage{Events: events, NextCursor: strconv.FormatInt(afterEventID, 10)}
		if page.Events == nil {
			page.Events = []contracts.SafeEvent{}
		}
		if len(page.Events) > 0 {
			page.NextCursor = page.Events[len(page.Events)-1].EventID
		}
		writeJSON(responseWriter, page)
	})
}

// verifiedOrganization returns the operator's organization or answers 401: the route guard always
// sets the operator, so its absence is never an allow.
func verifiedOrganization(responseWriter http.ResponseWriter, request *http.Request) (string, bool) {
	operator, verified := operatorcontext.FromContext(request.Context())
	if !verified || !uuidPattern.MatchString(operator.OrganizationID) {
		httpserver.WriteError(responseWriter, request, http.StatusUnauthorized, "unauthorized", "Missing or invalid operator context.")
		return "", false
	}
	return operator.OrganizationID, true
}

// runRequest returns the verified organization and the run id of the path. A malformed run id is
// not found, like another organization's run.
func runRequest(responseWriter http.ResponseWriter, request *http.Request) (string, string, bool) {
	organizationID, ok := verifiedOrganization(responseWriter, request)
	if !ok {
		return "", "", false
	}
	runID := request.PathValue("runId")
	if !uuidPattern.MatchString(runID) {
		writeRunNotFound(responseWriter, request)
		return "", "", false
	}
	return organizationID, runID, true
}

// eventPageQuery reads `after` (a non-negative event id, default 0) and `limit` (1 to 500,
// default 100). Anything else is 400.
func eventPageQuery(responseWriter http.ResponseWriter, request *http.Request) (int64, int, bool) {
	query := request.URL.Query()
	afterEventID := int64(0)
	if values, present := query["after"]; present {
		parsed, err := strconv.ParseInt(onlyValue(values), 10, 64)
		if err != nil || parsed < 0 {
			writeBadQuery(responseWriter, request)
			return 0, 0, false
		}
		afterEventID = parsed
	}
	limit, ok := pageLimit(responseWriter, request)
	return afterEventID, limit, ok
}

// pageLimit reads `limit`: 1 to 500, default 100.
func pageLimit(responseWriter http.ResponseWriter, request *http.Request) (int, bool) {
	values, present := request.URL.Query()["limit"]
	if !present {
		return defaultPageSize, true
	}
	limit, err := strconv.Atoi(onlyValue(values))
	if err != nil || limit < 1 || limit > maximumPageSize {
		writeBadQuery(responseWriter, request)
		return 0, false
	}
	return limit, true
}

// onlyValue returns the parameter's value, or "" (invalid) when it is repeated.
func onlyValue(values []string) string {
	if len(values) != 1 {
		return ""
	}
	return values[0]
}

// writeReadError maps a repository error: a run outside the organization is not found; anything
// else is unavailable, never an empty or partial answer.
func writeReadError(responseWriter http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrInvalid) || errors.Is(err, errRunNotFound) {
		writeRunNotFound(responseWriter, request)
		return
	}
	writeUnavailable(responseWriter, request)
}

func writeRunNotFound(responseWriter http.ResponseWriter, request *http.Request) {
	httpserver.WriteError(responseWriter, request, http.StatusNotFound, "not_found", "Run not found.")
}

func writeBadQuery(responseWriter http.ResponseWriter, request *http.Request) {
	httpserver.WriteError(responseWriter, request, http.StatusBadRequest, "bad_request", "The query parameters are not valid.")
}

func writeUnavailable(responseWriter http.ResponseWriter, request *http.Request) {
	httpserver.WriteError(responseWriter, request, http.StatusServiceUnavailable, "unavailable", "The records are unavailable.")
}

// writeJSON answers 200 with an uncached JSON body.
func writeJSON(responseWriter http.ResponseWriter, value any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Header().Set("Cache-Control", "no-store")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(value)
}

// readTransaction runs work in a short transaction that is always rolled back: reads never write.
func readTransaction(ctx context.Context, database Beginner, work func(pgx.Tx) error) error {
	if database == nil {
		return errors.New("reads: no database")
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	tx, err := database.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	return work(tx)
}
