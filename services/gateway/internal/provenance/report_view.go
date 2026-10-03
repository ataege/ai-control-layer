package provenance

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/health"
)

// ReportView is a stored report as the interface reads it (GO-37; the Go side's draft of X-64,
// "Report and lineage summary"). The interface renders the substantive content from it and its
// registered template, and reads the stored classification instead of computing a label.
type ReportView struct {
	ReportID              string                   `json:"reportId"`
	RunID                 string                   `json:"runId"`
	Version               int                      `json:"version"`
	Template              contracts.ReportTemplate `json:"template"`
	TemplateVersion       int                      `json:"templateVersion"`
	ProjectionRule        *string                  `json:"projectionRule"`
	ProjectionRuleVersion *int                     `json:"projectionRuleVersion"`
	Classification        string                   `json:"classification"`
	DestinationClass      string                   `json:"destinationClass"`
	Title                 string                   `json:"title"`
	ContentHash           string                   `json:"contentHash"`
	// Content is null when it is Internal only and the viewer may not read internal content.
	Content         *string          `json:"content"`
	ContentWithheld bool             `json:"contentWithheld"`
	Lineage         []LineageSummary `json:"lineage"`
}

// LineageSummary is one trusted source of a report: references and labels, never source values.
type LineageSummary struct {
	SourceKind     string   `json:"sourceKind"`
	SourceID       string   `json:"sourceId"`
	SourceVersion  int      `json:"sourceVersion"`
	Classification string   `json:"classification"`
	ConsumedFields []string `json:"consumedFields"`
}

// Viewer is the verified context of the caller, derived by the API layer from the operator
// context: never from the URL, the body or model output.
type Viewer struct {
	OrganizationID string
	// MayReadInternal: the viewer is an authorized internal reviewer of this organization
	// ("Restricted source content is shown only to authorized users").
	MayReadInternal bool
}

// ViewerFunc returns the verified viewer of a request, or false when there is none.
type ViewerFunc func(*http.Request) (Viewer, bool)

// Beginner starts a transaction: the gateway pool, or an enclosing transaction in tests.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// StoredReportRoutePattern is the route the API package mounts this handler on.
const StoredReportRoutePattern = "GET /internal/runs/{runId}/reports/{reportId}"

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// BuildReportView turns a stored report into its view for this viewer.
func BuildReportView(report StoredReport, viewer Viewer) ReportView {
	view := ReportView{
		ReportID: report.ID, RunID: report.RunID, Version: report.Version,
		Template: contracts.ReportTemplate(report.TemplateName), ProjectionRule: report.ProjectionRule,
		Classification: report.Classification, DestinationClass: report.DestinationClass,
		Title: report.Title, ContentHash: hex.EncodeToString(report.ContentHash[:]),
		Lineage: make([]LineageSummary, 0, len(report.Lineage)),
	}
	if template, registered := LookupTemplate(report.TemplateName); registered {
		view.TemplateVersion = template.Version
	}
	for _, entry := range report.Lineage {
		if entry.ProjectionRuleVersion != nil {
			version := *entry.ProjectionRuleVersion
			view.ProjectionRuleVersion = &version
		}
		view.Lineage = append(view.Lineage, LineageSummary{
			SourceKind: entry.Kind, SourceID: entry.ID, SourceVersion: entry.Version,
			Classification: entry.Classification, ConsumedFields: entry.ConsumedFields,
		})
	}
	if report.Classification == InternalOnly && !viewer.MayReadInternal {
		view.ContentWithheld = true
	} else {
		content := report.Content
		view.Content = &content
	}
	return view
}

// StoredReportHandler serves one stored report of the viewer's organization and the named run
// (GO-37). Another organization's or run's report is not found; ids are references, never
// authority. Errors use the shared ErrorResponse envelope.
func StoredReportHandler(database Beginner, viewerOf ViewerFunc) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		viewer, verified := viewerOf(request)
		if !verified || !uuidPattern.MatchString(viewer.OrganizationID) {
			writeReportError(responseWriter, request, http.StatusUnauthorized, "unauthorized", "Authentication required.")
			return
		}
		runID, reportID := request.PathValue("runId"), request.PathValue("reportId")
		if !uuidPattern.MatchString(runID) || !uuidPattern.MatchString(reportID) {
			writeReportError(responseWriter, request, http.StatusNotFound, "not_found", "Report not found.")
			return
		}
		view, err := readReportView(request.Context(), database, viewer, runID, reportID)
		if errors.Is(err, ErrReportNotFound) {
			writeReportError(responseWriter, request, http.StatusNotFound, "not_found", "Report not found.")
			return
		}
		if err != nil {
			writeReportError(responseWriter, request, http.StatusServiceUnavailable, "internal_error", "The report is unavailable.")
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Header().Set("Cache-Control", "no-store")
		responseWriter.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(responseWriter).Encode(view)
	})
}

// readReportView loads the report in a short read transaction that is always rolled back.
func readReportView(ctx context.Context, database Beginner, viewer Viewer, runID, reportID string) (ReportView, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := database.Begin(ctx)
	if err != nil {
		return ReportView{}, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	report, err := LoadReport(ctx, tx, viewer.OrganizationID, runID, reportID)
	if err != nil {
		return ReportView{}, err
	}
	return BuildReportView(report, viewer), nil
}

func writeReportError(responseWriter http.ResponseWriter, request *http.Request, statusCode int, code, message string) {
	// The RequestID middleware of the mounting server has set the header; it is echoed, never made up here.
	health.WriteJSON(responseWriter, statusCode, health.ErrorResponse{
		Error:      health.ErrorDetail{Code: code, Message: message},
		StatusCode: statusCode,
		RequestID:  responseWriter.Header().Get("x-request-id"),
		Timestamp:  time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Path:       request.URL.Path,
	})
}
