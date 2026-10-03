package provenance

// ExportDecision is the provenance answer for sending a stored report to a destination class.
// A denial carries a stable reason code; an allowed export still needs the passport, destination
// and exact-review checks of its callers.
type ExportDecision struct {
	Allowed    bool
	ReasonCode string
	// AlternativeTemplate names a permitted continuation when the report itself may not leave:
	// "Denial may identify an authorized alternative template without granting extra scope."
	AlternativeTemplate string
}

func deny(reasonCode string) ExportDecision {
	return ExportDecision{ReasonCode: reasonCode}
}

// AuthorizeExport decides from the stored report and its stored lineage only: the stored
// classification column, the title and any model-declared label carry no authority. Checks, in
// order: content integrity, complete lineage, registered template and projection versions, the
// inherited restriction of every source, the destination class, and current source versions.
func AuthorizeExport(report StoredReport, destinationClass string, currentVersions map[string]int) ExportDecision {
	if ContentHash(report.Content) != report.ContentHash {
		return deny(ReasonReportLineageMissing)
	}
	template, registered := LookupTemplate(report.TemplateName)
	if !registered || !projectionPolicyMatches(template, report.ProjectionPolicy) {
		return deny(ReasonTemplateNotAllowed)
	}
	if len(report.Lineage) == 0 {
		return deny(ReasonReportLineageMissing)
	}
	sources := make([]Source, 0, len(report.Lineage))
	for _, entry := range report.Lineage {
		if entry.TemplateName != template.Name || entry.TemplateVersion != template.Version {
			return deny(ReasonTemplateNotAllowed)
		}
		if !projectionMatches(template, entry) {
			return deny(ReasonTemplateNotAllowed)
		}
		sources = append(sources, entry.Source)
	}
	// The classification is derived again from the lineage; a stored or declared label never
	// substitutes for it.
	derived, err := DeriveClassification(template, sources)
	if err != nil {
		return deny(ReasonReportLineageMissing)
	}
	if destinationClass == DestinationRegisteredVendor && derived != VendorShareable {
		decision := deny(ReasonReportExportRestricted)
		decision.AlternativeTemplate = VendorReconciliationV1.Name
		return decision
	}
	if destinationClass != template.DestinationClass {
		return deny(ReasonDestinationNotAllowed)
	}
	for _, source := range sources {
		current, found := currentVersions[source.ID]
		if !found || current != source.Version {
			return deny(ReasonResourceVersionChanged)
		}
	}
	return ExportDecision{Allowed: true}
}

// projectionPolicyMatches checks the projection policy version stored with the report itself.
func projectionPolicyMatches(template Template, storedPolicy *string) bool {
	if template.Projection == nil {
		return storedPolicy == nil
	}
	return storedPolicy != nil && *storedPolicy == template.Projection.PolicyVersion
}

func projectionMatches(template Template, entry LineageEntry) bool {
	if template.Projection == nil {
		return entry.ProjectionRule == nil && entry.ProjectionRuleVersion == nil
	}
	return entry.ProjectionRule != nil && *entry.ProjectionRule == template.Projection.Name &&
		entry.ProjectionRuleVersion != nil && *entry.ProjectionRuleVersion == template.Projection.Version
}
