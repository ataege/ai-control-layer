package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/provenance"
)

// queueReportArguments are the X-09 arguments of queue_report: a stored report and a trusted
// recipient reference, never a raw address or rendered content.
type queueReportArguments struct {
	ReportID           string `json:"report_id"`
	RecipientReference string `json:"recipient_reference"`
}

// QueueResult is the model-facing result of queue_report; its fields are the GO-07 allowlist.
// The address and the queued content never appear.
type QueueResult struct {
	OutboxMessageID string `json:"outbox_message_id"`
	ReportID        string `json:"report_id"`
	Status          string `json:"status"`
}

// StatusQueuedSimulated labels the effect: a row in the simulated outbox; nothing is sent.
const StatusQueuedSimulated = "queued_simulated"

// queueReport queues the exact stored report to the trusted recipient as one simulated outbox
// row. It runs only under the bound exact-action approval the executor consumes (GO-45);
// approval never overrides a provenance denial ("ordinary approval cannot override the denial").
func queueReport(ctx context.Context, tx pgx.Tx, current scope, request EffectRequest) (adapterOutcome, error) {
	var arguments queueReportArguments
	if err := decodeArguments(request.CanonicalArguments, &arguments); err != nil {
		return adapterOutcome{}, err
	}
	if arguments.ReportID == "" || arguments.RecipientReference == "" {
		return adapterOutcome{}, fmt.Errorf("%w: report_id and recipient_reference are required", errPrecondition)
	}

	// "A generated report ID is not permission to queue an unrelated report."
	report, err := provenance.LoadReport(ctx, tx, current.organizationID, current.runID, arguments.ReportID)
	if errors.Is(err, provenance.ErrReportNotFound) {
		return failed(ReasonResourceOutOfScope), nil
	}
	if err != nil {
		return adapterOutcome{}, err
	}

	recipient, reason, err := resolveRecipient(ctx, tx, current, arguments.RecipientReference)
	if err != nil {
		return adapterOutcome{}, err
	}
	if reason != "" {
		return exportDenied(report, provenance.ExportDecision{ReasonCode: reason}), nil
	}

	// The export decision reads the stored lineage and the current source versions only.
	sourceIDs := make([]string, 0, len(report.Lineage))
	for _, entry := range report.Lineage {
		sourceIDs = append(sourceIDs, entry.ID)
	}
	versions, err := provenance.CurrentInvoiceVersions(ctx, tx, current.organizationID, sourceIDs)
	if err != nil {
		return adapterOutcome{}, err
	}
	decision := provenance.AuthorizeExport(report, provenance.DestinationRegisteredVendor, versions)
	if !decision.Allowed {
		return exportDenied(report, decision), nil
	}
	// The recipient must be the vendor of every source invoice of the report.
	sameVendor, err := sourcesBelongToVendor(ctx, tx, current.organizationID, sourceIDs, recipient.vendorID)
	if err != nil {
		return adapterOutcome{}, err
	}
	if !sameVendor {
		return exportDenied(report, provenance.ExportDecision{ReasonCode: ReasonDestinationNotAllowed}), nil
	}

	var outboxMessageID string
	err = tx.QueryRow(ctx,
		`INSERT INTO demo.outbox_messages (organization_id, action_id, report_id, report_content_hash, recipient)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		current.organizationID, request.ActionID, report.ID, report.ContentHash[:], recipient.address,
	).Scan(&outboxMessageID)
	if err != nil {
		return adapterOutcome{}, fmt.Errorf("tools: insert simulated outbox row: %w", err)
	}
	return succeeded(QueueResult{OutboxMessageID: outboxMessageID, ReportID: report.ID, Status: StatusQueuedSimulated},
		contracts.EventActionSucceeded, reportSummary(report, "passed", "outbox_message_queued")), nil
}

// exportDenied is a refused export: no outbox row, a report.export_denied event with the stable
// reason and, where the report itself may not leave, a report.safe_template_offered event naming
// the already-permitted alternative ("Denial may identify an authorized alternative template
// without granting extra scope").
func exportDenied(report provenance.StoredReport, decision provenance.ExportDecision) adapterOutcome {
	lineageCheck := "passed"
	switch decision.ReasonCode {
	case ReasonReportLineageMissing:
		lineageCheck = "missing"
	case ReasonResourceVersionChanged, ReasonTemplateNotAllowed:
		lineageCheck = "failed"
	}
	deny := contracts.DecisionDeny
	denied := reportSummary(report, lineageCheck, "none")
	outcome := adapterOutcome{
		result: EffectResult{Outcome: OutcomeFailed, ReasonCode: decision.ReasonCode},
		events: []eventRecord{{eventType: contracts.EventReportExportDenied, decision: &deny, summary: denied}},
	}
	if decision.AlternativeTemplate != "" {
		alternative := contracts.ReportTemplate(decision.AlternativeTemplate)
		denied.AlternativeTemplate = &alternative
		outcome.events[0].summary = denied
		offered := reportSummary(report, lineageCheck, "none")
		offered.AlternativeTemplate = &alternative
		outcome.events = append(outcome.events, eventRecord{eventType: contracts.EventReportSafeTemplateOffered, summary: offered})
	}
	return outcome
}

// reportSummary is the masked metadata of a report event: references and Go-derived labels.
func reportSummary(report provenance.StoredReport, lineageCheck, effect string) contracts.MaskedSummary {
	template := contracts.ReportTemplate(report.TemplateName)
	reportID, classification := report.ID, report.Classification
	return contracts.MaskedSummary{
		ReportID: &reportID, Template: &template, Classification: &classification,
		LineageCheck: text(lineageCheck), Effect: text(effect),
	}
}

// sourcesBelongToVendor reports whether every source invoice is the given vendor's.
func sourcesBelongToVendor(ctx context.Context, tx pgx.Tx, organizationID string, invoiceIDs []string, vendorID string) (bool, error) {
	var mismatched int
	err := tx.QueryRow(ctx,
		`SELECT count(*) FROM demo.invoices
		  WHERE organization_id = $1 AND id = ANY($2) AND vendor_id <> $3`,
		organizationID, invoiceIDs, vendorID,
	).Scan(&mismatched)
	if err != nil {
		return false, fmt.Errorf("tools: check report vendor: %w", err)
	}
	return mismatched == 0 && len(invoiceIDs) > 0, nil
}
