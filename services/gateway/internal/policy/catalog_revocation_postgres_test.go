package policy

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/testdb"
)

// fakeSnapshots stands in for the active catalog: the test sets the enabled templates or an error.
type fakeSnapshots struct {
	snapshot catalog.Snapshot
	err      error
}

func (snapshots fakeSnapshots) Active(context.Context, catalog.Querier) (catalog.Snapshot, error) {
	return snapshots.snapshot, snapshots.err
}

func enabling(revision int64, templates ...contracts.ReportTemplate) fakeSnapshots {
	return fakeSnapshots{snapshot: catalog.Snapshot{RevisionID: revision, Limits: catalog.Limits{EnabledTemplates: templates}}}
}

// scopeReaderOn reads the review world's stored passport with the given catalog source.
func scopeReaderOn(world *reviewWorld, snapshots activeSnapshots) *PassportScopeReader {
	return &PassportScopeReader{pool: world.pool, repository: repository.New(world.pool), catalogs: snapshots}
}

// GO-70/GO-72: a template disabled in the active catalog after admission is no longer in the
// gate's scope; the catalog only narrows and never widens the passport.
func TestTemplateRevokedThroughTheCatalogNarrowsTheScope(t *testing.T) {
	world := openReviewWorld(t)
	ctx := context.Background()

	// The passport allows only the vendor template; a catalog enabling both adds nothing.
	both := enabling(1, contracts.TemplateVendorReconciliation, contracts.TemplateInternalInvestigation)
	scope, err := scopeReaderOn(world, both).LoadScope(ctx, world.run)
	if err != nil || !slices.Equal(scope.AllowedTemplates, []string{string(contracts.TemplateVendorReconciliation)}) {
		t.Fatalf("still enabled: templates %v err %v; want only the passport's vendor template", scope.AllowedTemplates, err)
	}

	// The judge removes the vendor template from reports.enabled_templates.
	revoked := enabling(2, contracts.TemplateInternalInvestigation)
	scope, err = scopeReaderOn(world, revoked).LoadScope(ctx, world.run)
	if err != nil || len(scope.AllowedTemplates) != 0 {
		t.Fatalf("revoked: templates %v err %v; want none", scope.AllowedTemplates, err)
	}

	// A new create_report with the revoked template is refused with the existing reason code,
	// and the denial feedback no longer suggests it as an alternative.
	gate := NewGate(scopeReaderOn(world, revoked), NewPostgresRecorder(world.pool), NewPostgresRelationships(world.pool), nil)
	world.nextStep++
	decision := gate.Evaluate(ctx, world.run, Proposal{ActionID: testdb.ID(t), StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t),
		Tool: "create_report", RawArguments: json.RawMessage(`{"template":"vendor_reconciliation_v1","source_invoice_ids":["` + world.invoiceID + `"]}`)})
	if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonTemplateNotAllowed {
		t.Fatalf("create_report with a revoked template = %s/%s, want deny/%s", decision.Outcome, decision.ReasonCode, ReasonTemplateNotAllowed)
	}
	feedback := BuildDenialFeedback(Decision{Outcome: OutcomeDeny, ReasonCode: ReasonReportExportRestricted,
		AlternativeTemplate: string(contracts.TemplateVendorReconciliation)}, scope)
	if feedback.AlternativeTemplate != "" {
		t.Errorf("feedback suggests the revoked template %q", feedback.AlternativeTemplate)
	}
}

// Without an enforceable active catalog there is no scope and no revision: the gate denies.
func TestTemplateRevokedThroughTheCatalogFailsClosedWithoutACatalog(t *testing.T) {
	world := openReviewWorld(t)
	reader := scopeReaderOn(world, fakeSnapshots{err: catalog.ErrUnavailable})
	if _, err := reader.LoadScope(context.Background(), world.run); !errors.Is(err, ErrScopeUnavailable) {
		t.Fatalf("LoadScope without a catalog: %v, want ErrScopeUnavailable", err)
	}
	if _, err := reader.ActiveCatalogRevision(context.Background()); !errors.Is(err, ErrScopeUnavailable) {
		t.Fatalf("ActiveCatalogRevision without a catalog: %v, want ErrScopeUnavailable", err)
	}
	gate := NewGate(reader, NewPostgresRecorder(world.pool), NewPostgresRelationships(world.pool), nil)
	world.nextStep++
	decision := gate.Evaluate(context.Background(), world.run, Proposal{ActionID: testdb.ID(t), StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t),
		Tool: "read_invoice", RawArguments: json.RawMessage(`{"invoice_id":"` + world.invoiceID + `"}`)})
	if decision.Outcome == OutcomeAllow || decision.Outcome == OutcomeApprovalRequired {
		t.Fatalf("decision without a catalog = %s/%s, want a denial", decision.Outcome, decision.ReasonCode)
	}
}
