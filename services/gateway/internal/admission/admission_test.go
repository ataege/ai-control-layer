package admission

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/testdb"
)

const testCatalogContent = `{
	"schema_version": 1,
	"allowed_models": ["qwen3.5:4b"],
	"budgets": {"calls_total": 24, "calls_agent": 12, "calls_security": 12, "tokens_total": 20000,
		"request_timeout_seconds": 20, "local_max_concurrency": 2, "run_expiry_minutes": 15,
		"tool_attempts": 12, "corrections": 2, "agent_output_tokens": 512},
	"reports": {"enabled_templates": ["internal_investigation_v1", "vendor_reconciliation_v1"]},
	"controls": {}
}`

// admissionFixture is one isolated organization with Atlas-like demo records and an active
// catalog, all inside one outer transaction that is rolled back when the test ends.
type admissionFixture struct {
	outer          pgx.Tx
	admitter       *Admitter
	operator       contracts.OperatorContext
	vendorID       string
	otherVendorID  string
	silentVendorID string
	invoiceIDs     []string
	otherInvoiceID string
	foreignInvoice string
	revisionID     int64
	now            time.Time
}

func exec(t *testing.T, transaction pgx.Tx, sql string, arguments ...any) {
	t.Helper()
	if _, err := transaction.Exec(context.Background(), sql, arguments...); err != nil {
		t.Fatalf("fixture statement failed: %v", err)
	}
}

func activateCatalog(t *testing.T, outer pgx.Tx, content string) int64 {
	t.Helper()
	var revisionID int64
	err := outer.QueryRow(context.Background(), `INSERT INTO app.control_catalog_revisions
		(schema_version, source_file_name, source_text, file_digest, content, import_source)
		VALUES (1, 'policy.yaml', 'admission test', repeat('a', 64), $1, 'command') RETURNING id`, content).Scan(&revisionID)
	if err != nil {
		t.Fatalf("insert catalog revision: %v", err)
	}
	exec(t, outer, `INSERT INTO app.control_catalog_pointer (id, active_revision_id) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET active_revision_id = EXCLUDED.active_revision_id`, revisionID)
	return revisionID
}

func newFixture(t *testing.T) *admissionFixture {
	t.Helper()
	pool := testdb.Open(t)
	outer, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = outer.Rollback(context.Background()) })

	organizationID, otherOrganizationID := testdb.ID(t), testdb.ID(t)
	suffix := testdb.ID(t)
	fixture := &admissionFixture{
		outer:          outer,
		operator:       contracts.OperatorContext{UserID: testdb.ID(t), OrganizationID: organizationID, Roles: []string{"operator"}},
		vendorID:       "vendor_atlas_" + suffix,
		otherVendorID:  "vendor_other_" + suffix,
		silentVendorID: "vendor_silent_" + suffix,
		invoiceIDs:     []string{"invoice_a01_" + suffix, "invoice_a02_" + suffix},
		otherInvoiceID: "invoice_b01_" + suffix,
		foreignInvoice: "invoice_x01_" + suffix,
		now:            time.Date(2026, 10, 3, 12, 0, 0, 123456789, time.UTC),
	}
	insertVendor := `INSERT INTO demo.vendors (id, organization_id, name, registered_reporting_address) VALUES ($1, $2, $3, $4)`
	exec(t, outer, insertVendor, fixture.vendorID, organizationID, "Atlas", "reports@atlas.example.com")
	exec(t, outer, insertVendor, fixture.otherVendorID, organizationID, "Borealis", "reports@borealis.example.com")
	exec(t, outer, insertVendor, fixture.silentVendorID, organizationID, "Silent", nil)
	exec(t, outer, insertVendor, "vendor_foreign_"+suffix, otherOrganizationID, "Foreign", "x@foreign.example.com")
	insertInvoice := `INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency,
		total_minor_units, issued_on, due_on) VALUES ($1, $2, $3, 'INV104', 'EUR', 125000, '2026-09-01', '2026-10-31')`
	for _, invoiceID := range fixture.invoiceIDs {
		exec(t, outer, insertInvoice, invoiceID, organizationID, fixture.vendorID)
	}
	exec(t, outer, insertInvoice, fixture.otherInvoiceID, organizationID, fixture.otherVendorID)
	exec(t, outer, insertInvoice, fixture.foreignInvoice, otherOrganizationID, "vendor_foreign_"+suffix)
	exec(t, outer, insertInvoice, "invoice_s01_"+suffix, organizationID, fixture.silentVendorID)
	fixture.revisionID = activateCatalog(t, outer, testCatalogContent)

	fixture.admitter = New(repository.New(outer))
	fixture.admitter.now = func() time.Time { return fixture.now }
	return fixture
}

func (fixture *admissionFixture) request() contracts.StartRunRequest {
	return contracts.StartRunRequest{
		Template:    TaskTemplateReconcileAtlas,
		VendorID:    &fixture.vendorID,
		InvoiceIDs:  append([]string(nil), fixture.invoiceIDs...),
		Destination: fixture.vendorID,
	}
}

func (fixture *admissionFixture) count(t *testing.T, sql string) int {
	t.Helper()
	var count int
	if err := fixture.outer.QueryRow(context.Background(), sql, fixture.operator.OrganizationID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	return count
}

func (fixture *admissionFixture) assertNothingAdmitted(t *testing.T) {
	t.Helper()
	for _, table := range []string{"runtime.passports", "runtime.runs", "runtime.jobs"} {
		if count := fixture.count(t, "SELECT count(*) FROM "+table+" WHERE organization_id = $1"); count != 0 {
			t.Errorf("%s has %d rows after a rejected admission", table, count)
		}
	}
}

func TestPostgresAdmissionIssuesPassportRunAndJob(t *testing.T) {
	fixture := newFixture(t)
	request := fixture.request()
	request.ApprovalRequirement = pointer(ApprovalRuleReviewQueueReport)
	request.Limits = &contracts.StartRunLimits{ModelCalls: pointer(int64(10)), TimeoutSeconds: pointer(int64(600))}

	passport, err := fixture.admitter.Admit(context.Background(), fixture.operator, request)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	issuedAt := fixture.now.Truncate(time.Microsecond)
	if passport.OrganizationID != fixture.operator.OrganizationID || passport.ActorID != fixture.operator.UserID ||
		passport.AdmissionCatalogRevisionID != fixture.revisionID || !passport.IssuedAt.Equal(issuedAt) ||
		!passport.ExpiresAt.Equal(issuedAt.Add(10*time.Minute)) {
		t.Errorf("identity or lifetime wrong: %+v", passport)
	}
	scope := passport.Scope
	if !reflect.DeepEqual(scope.InvoiceIDs, fixture.invoiceIDs) || !reflect.DeepEqual(scope.VendorIDs, []string{fixture.vendorID}) ||
		!reflect.DeepEqual(scope.RecipientReferences, []string{"recipient:" + passport.RunID + ":" + fixture.vendorID}) ||
		!reflect.DeepEqual(scope.ReportTemplates, contracts.ReportTemplates) ||
		!reflect.DeepEqual(scope.ProjectionRules, []string{"vendor_invoice_fields_v1"}) ||
		!reflect.DeepEqual(scope.ApprovalRequiredTools, []contracts.ToolName{contracts.ToolQueueReport}) || !scope.InternalNoteReadable {
		t.Errorf("scope wrong: %+v", scope)
	}
	limits := passport.Limits
	if limits.CallsTotal != 10 || limits.CallsAgent != 10 || limits.CallsSecurity != 10 || limits.TokensTotal != 20000 ||
		limits.RunExpiryMinutes != 15 || limits.TokensAgent != nil {
		t.Errorf("limits wrong: %+v", limits)
	}

	stored, err := repository.New(fixture.outer).Passport(context.Background(), fixture.operator.OrganizationID, passport.RunID)
	if err != nil || !reflect.DeepEqual(stored, passport) {
		t.Errorf("stored passport differs (err %v)\nwant %+v\ngot  %+v", err, passport, stored)
	}
	for table, want := range map[string]int{"runtime.passports": 1, "runtime.runs": 1, "runtime.jobs": 1} {
		if count := fixture.count(t, "SELECT count(*) FROM "+table+" WHERE organization_id = $1"); count != want {
			t.Errorf("%s has %d rows, want %d", table, count, want)
		}
	}
	if count := fixture.count(t, `SELECT count(*) FROM runtime.jobs WHERE organization_id = $1 AND status = 'queued' AND kind = 'agent_step'`); count != 1 {
		t.Error("the first job is not a queued agent_step")
	}
	if count := fixture.count(t, `SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1 AND event_type = 'run.queued'`); count != 1 {
		t.Error("no run.queued event")
	}
}

func TestPostgresAdmissionRejectsWhatExceedsAuthority(t *testing.T) {
	cases := map[string]struct {
		change   func(fixture *admissionFixture, request *contracts.StartRunRequest)
		wantCode contracts.ReasonCode
	}{
		"another organization's invoice": {func(fixture *admissionFixture, request *contracts.StartRunRequest) {
			request.InvoiceIDs = append(request.InvoiceIDs, fixture.foreignInvoice)
		}, contracts.ReasonResourceOutOfScope},
		"unknown invoice": {func(_ *admissionFixture, request *contracts.StartRunRequest) {
			request.InvoiceIDs = append(request.InvoiceIDs, "invoice_missing")
		}, contracts.ReasonResourceOutOfScope},
		"invoices of two vendors": {func(fixture *admissionFixture, request *contracts.StartRunRequest) {
			request.InvoiceIDs = append(request.InvoiceIDs, fixture.otherInvoiceID)
		}, contracts.ReasonResourceOutOfScope},
		"vendor mismatch": {func(fixture *admissionFixture, request *contracts.StartRunRequest) {
			request.VendorID = &fixture.otherVendorID
		}, contracts.ReasonResourceOutOfScope},
		"destination of another vendor": {func(fixture *admissionFixture, request *contracts.StartRunRequest) {
			request.Destination = fixture.otherVendorID
		}, contracts.ReasonDestinationNotAllowed},
		"destination without a registered address": {func(fixture *admissionFixture, request *contracts.StartRunRequest) {
			request.InvoiceIDs = []string{strings.Replace(fixture.otherInvoiceID, "invoice_b01_", "invoice_s01_", 1)}
			request.VendorID = &fixture.silentVendorID
			request.Destination = fixture.silentVendorID
		}, contracts.ReasonDestinationNotAllowed},
		"model calls above the catalog": {func(_ *admissionFixture, request *contracts.StartRunRequest) {
			request.Limits = &contracts.StartRunLimits{ModelCalls: pointer(int64(25))}
		}, contracts.ReasonLimitNotAllowed},
		"run time above the catalog": {func(_ *admissionFixture, request *contracts.StartRunRequest) {
			request.Limits = &contracts.StartRunLimits{TimeoutSeconds: pointer(int64(15*60 + 1))}
		}, contracts.ReasonLimitNotAllowed},
		"unknown task template": {func(_ *admissionFixture, request *contracts.StartRunRequest) {
			request.Template = "reconcile_everything_v1"
		}, contracts.ReasonTemplateNotAllowed},
		"weaker approval rule": {func(_ *admissionFixture, request *contracts.StartRunRequest) {
			request.ApprovalRequirement = pointer("none")
		}, contracts.ReasonInvalidArguments},
		"repeated invoice": {func(fixture *admissionFixture, request *contracts.StartRunRequest) {
			request.InvoiceIDs = append(request.InvoiceIDs, fixture.invoiceIDs[0])
		}, contracts.ReasonInvalidArguments},
		"no invoices": {func(_ *admissionFixture, request *contracts.StartRunRequest) {
			request.InvoiceIDs = nil
		}, contracts.ReasonInvalidArguments},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newFixture(t)
			request := fixture.request()
			testCase.change(fixture, &request)
			_, err := fixture.admitter.Admit(context.Background(), fixture.operator, request)
			var rejection *Rejection
			if !errors.As(err, &rejection) || rejection.Code != testCase.wantCode || rejection.Message == "" {
				t.Fatalf("got %v, want rejection %s", err, testCase.wantCode)
			}
			fixture.assertNothingAdmitted(t)
			if count := fixture.count(t, `SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1
				AND event_type = 'admission.rejected' AND reason_code = '`+string(testCase.wantCode)+`' AND run_id IS NULL`); count != 1 {
				t.Errorf("admission.rejected events = %d, want 1", count)
			}
		})
	}
}

func TestPostgresAdmissionWithoutActiveCatalogIsUnavailable(t *testing.T) {
	fixture := newFixture(t)
	exec(t, fixture.outer, `UPDATE app.control_catalog_pointer SET active_revision_id = NULL WHERE id = 1`)
	if _, err := fixture.admitter.Admit(context.Background(), fixture.operator, fixture.request()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
	fixture.assertNothingAdmitted(t)
}

func TestPostgresAdmissionRejectsAnInvalidCatalog(t *testing.T) {
	fixture := newFixture(t)
	activateCatalog(t, fixture.outer, strings.Replace(testCatalogContent, `"calls_agent": 12`, `"calls_agent": 30`, 1))
	if _, err := fixture.admitter.Admit(context.Background(), fixture.operator, fixture.request()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
	fixture.assertNothingAdmitted(t)
}

func TestPostgresAdmissionNarrowedCatalogNarrowsTemplates(t *testing.T) {
	fixture := newFixture(t)
	activateCatalog(t, fixture.outer, strings.Replace(testCatalogContent,
		`["internal_investigation_v1", "vendor_reconciliation_v1"]`, `["internal_investigation_v1"]`, 1))
	passport, err := fixture.admitter.Admit(context.Background(), fixture.operator, fixture.request())
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if !reflect.DeepEqual(passport.Scope.ReportTemplates, []contracts.ReportTemplate{contracts.TemplateInternalInvestigation}) ||
		len(passport.Scope.ProjectionRules) != 0 {
		t.Errorf("templates %v projections %v", passport.Scope.ReportTemplates, passport.Scope.ProjectionRules)
	}
}

func TestParseCatalogFailsClosed(t *testing.T) {
	broken := map[string]string{
		"not json":              `{`,
		"wrong schema version":  strings.Replace(testCatalogContent, `"schema_version": 1`, `"schema_version": 2`, 1),
		"missing calls total":   strings.Replace(testCatalogContent, `"calls_total": 24, `, ``, 1),
		"zero tool attempts":    strings.Replace(testCatalogContent, `"tool_attempts": 12`, `"tool_attempts": 0`, 1),
		"negative corrections":  strings.Replace(testCatalogContent, `"corrections": 2`, `"corrections": -1`, 1),
		"timeout beyond expiry": strings.Replace(testCatalogContent, `"request_timeout_seconds": 20`, `"request_timeout_seconds": 900`, 1),
		"no models":             strings.Replace(testCatalogContent, `["qwen3.5:4b"]`, `[]`, 1),
		"unknown template":      strings.Replace(testCatalogContent, `"vendor_reconciliation_v1"]`, `"public_v1"]`, 1),
		"no reports group":      strings.Replace(testCatalogContent, `"reports": {"enabled_templates": ["internal_investigation_v1", "vendor_reconciliation_v1"]},`, ``, 1),
	}
	for name, content := range broken {
		if _, err := parseCatalog(1, []byte(content)); !errors.Is(err, ErrCatalogUnavailable) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if _, err := parseCatalog(1, []byte(testCatalogContent)); err != nil {
		t.Errorf("valid catalog rejected: %v", err)
	}
}

func TestNewUUIDIsAVersion4UUID(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		value := newUUID()
		if len(value) != 36 || value[14] != '4' || !strings.ContainsRune("89ab", rune(value[19])) || seen[value] {
			t.Fatalf("bad or repeated uuid %q", value)
		}
		seen[value] = true
	}
}

func TestPostgresAdmissionFaultBeforeCommitLeavesNothing(t *testing.T) {
	fixture := newFixture(t)
	// A test-only trigger fails the last write of the admission transaction, the run.queued event;
	// it is dropped again when the outer transaction rolls back.
	exec(t, fixture.outer, `CREATE FUNCTION pg_temp.fail_run_queued() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected admission fault'; END; $$`)
	exec(t, fixture.outer, `CREATE TRIGGER admission_fault BEFORE INSERT ON runtime.audit_events
		FOR EACH ROW WHEN (NEW.event_type = 'run.queued') EXECUTE FUNCTION pg_temp.fail_run_queued()`)
	if _, err := fixture.admitter.Admit(context.Background(), fixture.operator, fixture.request()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
	fixture.assertNothingAdmitted(t)
}
