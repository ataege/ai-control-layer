package provenance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"starter/services/gateway/internal/testdb"
)

func serveReport(t *testing.T, world *reportWorld, viewer *Viewer, runID, reportID string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(StoredReportRoutePattern, StoredReportHandler(world.tx, func(*http.Request) (Viewer, bool) {
		if viewer == nil {
			return Viewer{}, false
		}
		return *viewer, true
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/runs/"+runID+"/reports/"+reportID, nil)
	request.Header.Set("x-request-id", "test-request")
	recorder.Header().Set("x-request-id", "test-request")
	mux.ServeHTTP(recorder, request)
	return recorder
}

func storeInternalReport(t *testing.T, world *reportWorld) StoredReport {
	t.Helper()
	stored, err := StoreReport(context.Background(), world.tx, NewReport{
		OrganizationID: world.organizationID, RunID: world.runID, CreatedByActionID: world.action(t, 1),
		Template: InternalInvestigationV1, Title: "Internal investigation", Content: "internal body with the note",
		Sources: []Source{invoiceSource(world.invoiceA01, InternalOnly, FieldExternalReference, FieldInternalNote)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func TestStoredReportHandlerServesTheViewersOrganizationOnly(t *testing.T) {
	world := openReportWorld(t)
	stored := storeInternalReport(t, world)
	reviewer := &Viewer{OrganizationID: world.organizationID, MayReadInternal: true}

	response := serveReport(t, world, reviewer, world.runID, stored.ID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", response.Code, response.Body)
	}
	decoder := json.NewDecoder(strings.NewReader(response.Body.String()))
	decoder.DisallowUnknownFields()
	var view ReportView
	if err := decoder.Decode(&view); err != nil {
		t.Fatalf("strict decode: %v", err)
	}
	if view.Classification != InternalOnly || view.Content == nil || view.TemplateVersion != 1 ||
		len(view.Lineage) != 1 || view.Lineage[0].Classification != InternalOnly || view.ContentWithheld {
		t.Fatalf("view = %+v", view)
	}

	other := &Viewer{OrganizationID: testdb.ID(t), MayReadInternal: true}
	for name, response := range map[string]*httptest.ResponseRecorder{
		"other organization": serveReport(t, world, other, world.runID, stored.ID),
		"other run":          serveReport(t, world, reviewer, testdb.ID(t), stored.ID),
		"not an id":          serveReport(t, world, reviewer, world.runID, "invoice_A01"),
	} {
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "internal body") {
			t.Errorf("%s: status %d, body %s", name, response.Code, response.Body)
		}
	}
	if response := serveReport(t, world, nil, world.runID, stored.ID); response.Code != http.StatusUnauthorized {
		t.Errorf("no verified viewer: status %d", response.Code)
	}
}

func TestStoredReportHandlerWithholdsInternalContentFromOtherViewers(t *testing.T) {
	world := openReportWorld(t)
	stored := storeInternalReport(t, world)
	response := serveReport(t, world, &Viewer{OrganizationID: world.organizationID}, world.runID, stored.ID)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var view ReportView
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if !view.ContentWithheld || view.Content != nil || strings.Contains(response.Body.String(), "internal body") {
		t.Fatalf("internal content reached a viewer without internal access: %s", response.Body)
	}
	if view.Classification != InternalOnly || len(view.Lineage) != 1 {
		t.Fatalf("labels and lineage must still be shown: %+v", view)
	}
}
