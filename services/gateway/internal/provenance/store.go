package provenance

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// NewReport is a report Go rendered from trusted sources, ready to store with its lineage.
type NewReport struct {
	OrganizationID, RunID, CreatedByActionID string
	Template                                 Template
	Sources                                  []Source
	Title, Content                           string
}

// StoredReport is a report with its lineage, as read back from the database.
type StoredReport struct {
	ID, OrganizationID, RunID        string
	Version                          int
	TemplateName                     string
	ProjectionRule, ProjectionPolicy *string
	Classification, DestinationClass string
	Title, Content                   string
	ContentHash                      [32]byte
	Lineage                          []LineageEntry
}

// LineageEntry is one runtime.report_lineage row.
type LineageEntry struct {
	Source
	TemplateName          string
	TemplateVersion       int
	ProjectionRule        *string
	ProjectionRuleVersion *int
}

// ContentHash is SHA-256 of the exact UTF-8 bytes stored (demo.reports checks the same).
func ContentHash(content string) [32]byte {
	return sha256.Sum256([]byte(content))
}

// StoreReport derives the classification from the trusted sources and stores the report and
// its lineage in tx, so "an artifact cannot exist without its restrictions". Nothing is stored
// when the lineage is incomplete.
func StoreReport(ctx context.Context, tx pgx.Tx, report NewReport) (StoredReport, error) {
	classification, err := DeriveClassification(report.Template, report.Sources)
	if err != nil {
		return StoredReport{}, err
	}
	// Store the registered definition's destination and projection, never the caller's copy.
	report.Template = registeredTemplates[report.Template.Name]
	var projectionRule, projectionPolicy *string
	var projectionVersion *int
	if report.Template.Projection != nil {
		projectionRule = &report.Template.Projection.Name
		projectionPolicy = &report.Template.Projection.PolicyVersion
		projectionVersion = &report.Template.Projection.Version
	}
	hash := ContentHash(report.Content)

	stored := StoredReport{
		OrganizationID: report.OrganizationID, RunID: report.RunID, Version: 1,
		TemplateName: report.Template.Name, ProjectionRule: projectionRule, ProjectionPolicy: projectionPolicy,
		Classification: classification, DestinationClass: report.Template.DestinationClass,
		Title: report.Title, Content: report.Content, ContentHash: hash,
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO demo.reports (organization_id, run_id, created_by_action_id, version, template, projection_rule,
		   projection_policy_version, classification, destination_class, title, content, content_hash)
		 VALUES ($1, $2, $3, 1, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		report.OrganizationID, report.RunID, report.CreatedByActionID, report.Template.Name, projectionRule,
		projectionPolicy, classification, report.Template.DestinationClass, report.Title, report.Content, hash[:],
	).Scan(&stored.ID)
	if err != nil {
		return StoredReport{}, fmt.Errorf("provenance: store report: %w", err)
	}
	for _, source := range report.Sources {
		entry := LineageEntry{Source: source, TemplateName: report.Template.Name, TemplateVersion: report.Template.Version,
			ProjectionRule: projectionRule, ProjectionRuleVersion: projectionVersion}
		_, err := tx.Exec(ctx,
			`INSERT INTO runtime.report_lineage (organization_id, run_id, report_id, source_kind, source_id, source_version,
			   source_classification, consumed_fields, template, template_version, projection_rule, projection_rule_version)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			report.OrganizationID, report.RunID, stored.ID, source.Kind, source.ID, source.Version,
			source.Classification, source.ConsumedFields, report.Template.Name, report.Template.Version,
			projectionRule, projectionVersion,
		)
		if err != nil {
			return StoredReport{}, fmt.Errorf("provenance: store lineage: %w", err)
		}
		stored.Lineage = append(stored.Lineage, entry)
	}
	return stored, nil
}

// ErrReportNotFound means no report with that id exists in this organization and run.
var ErrReportNotFound = errors.New("provenance: report not found")

// LoadReport reads a report of this organization and run with its stored lineage. A generated
// report id is not permission: another run's or organization's report is not found.
func LoadReport(ctx context.Context, tx pgx.Tx, organizationID, runID, reportID string) (StoredReport, error) {
	var report StoredReport
	var hash []byte
	err := tx.QueryRow(ctx,
		`SELECT id, organization_id, run_id, version, template, projection_rule, projection_policy_version,
		        classification, destination_class, title, content, content_hash
		   FROM demo.reports
		  WHERE id = $1 AND organization_id = $2 AND run_id = $3`,
		reportID, organizationID, runID,
	).Scan(&report.ID, &report.OrganizationID, &report.RunID, &report.Version, &report.TemplateName,
		&report.ProjectionRule, &report.ProjectionPolicy, &report.Classification, &report.DestinationClass,
		&report.Title, &report.Content, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredReport{}, ErrReportNotFound
	}
	if err != nil {
		return StoredReport{}, fmt.Errorf("provenance: load report: %w", err)
	}
	if len(hash) != len(report.ContentHash) {
		return StoredReport{}, fmt.Errorf("provenance: stored hash has %d bytes", len(hash))
	}
	copy(report.ContentHash[:], hash)

	rows, err := tx.Query(ctx,
		`SELECT source_kind, source_id, source_version, source_classification, consumed_fields,
		        template, template_version, projection_rule, projection_rule_version
		   FROM runtime.report_lineage
		  WHERE report_id = $1 AND organization_id = $2 AND run_id = $3
		  ORDER BY source_kind, source_id`,
		reportID, organizationID, runID,
	)
	if err != nil {
		return StoredReport{}, fmt.Errorf("provenance: load lineage: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var entry LineageEntry
		if err := rows.Scan(&entry.Kind, &entry.ID, &entry.Version, &entry.Classification, &entry.ConsumedFields,
			&entry.TemplateName, &entry.TemplateVersion, &entry.ProjectionRule, &entry.ProjectionRuleVersion); err != nil {
			return StoredReport{}, fmt.Errorf("provenance: read lineage: %w", err)
		}
		report.Lineage = append(report.Lineage, entry)
	}
	if err := rows.Err(); err != nil {
		return StoredReport{}, fmt.Errorf("provenance: read lineage: %w", err)
	}
	return report, nil
}

// CurrentInvoiceVersions returns the current version of each invoice of the organization.
func CurrentInvoiceVersions(ctx context.Context, tx pgx.Tx, organizationID string, invoiceIDs []string) (map[string]int, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, version FROM demo.invoices WHERE organization_id = $1 AND id = ANY($2)`,
		organizationID, invoiceIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("provenance: read source versions: %w", err)
	}
	defer rows.Close()
	versions := make(map[string]int, len(invoiceIDs))
	for rows.Next() {
		var invoiceID string
		var version int
		if err := rows.Scan(&invoiceID, &version); err != nil {
			return nil, fmt.Errorf("provenance: read source version: %w", err)
		}
		versions[invoiceID] = version
	}
	return versions, rows.Err()
}
