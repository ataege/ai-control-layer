import type { ReportTemplate, ReportView } from "@workspace/contracts";
import { fetchJson, type FetchJsonOptions, type FetchJsonResult } from "../fetch-json";

// The two report templates the gateway registers. A report naming any other template is not shown:
// the interface renders only what a registered template produced (WEB-12).
export const REGISTERED_TEMPLATES: readonly ReportTemplate[] = [
  "internal_investigation_v1",
  "vendor_reconciliation_v1",
];

const CLASSIFICATIONS = ["internal_only", "vendor_shareable"] as const;
const DESTINATION_CLASSES = ["internal_reviewers", "registered_vendor_recipient"] as const;
const HASH_PATTERN = /^[0-9a-f]{64}$/;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 1;
}

function isLineageEntry(value: unknown): boolean {
  return (
    isRecord(value) &&
    value.sourceKind === "invoice" &&
    typeof value.sourceId === "string" &&
    value.sourceId !== "" &&
    isPositiveInteger(value.sourceVersion) &&
    CLASSIFICATIONS.includes(value.classification as (typeof CLASSIFICATIONS)[number]) &&
    Array.isArray(value.consumedFields) &&
    value.consumedFields.length > 0 &&
    value.consumedFields.every((field) => typeof field === "string" && field !== "")
  );
}

/**
 * Structural check of a stored report (X-64, `ReportView`). It never repairs or derives anything:
 * a report outside the registered templates, with an inconsistent withheld state or without a
 * source trail is refused, so the page shows an error instead of free text.
 */
export function isReportView(data: unknown): data is ReportView {
  if (!isRecord(data)) return false;
  const withheld = data.contentWithheld;
  return (
    typeof data.reportId === "string" &&
    typeof data.runId === "string" &&
    isPositiveInteger(data.version) &&
    REGISTERED_TEMPLATES.includes(data.template as ReportTemplate) &&
    isPositiveInteger(data.templateVersion) &&
    (data.projectionRule === null || typeof data.projectionRule === "string") &&
    (data.projectionRuleVersion === null || isPositiveInteger(data.projectionRuleVersion)) &&
    CLASSIFICATIONS.includes(data.classification as (typeof CLASSIFICATIONS)[number]) &&
    DESTINATION_CLASSES.includes(data.destinationClass as (typeof DESTINATION_CLASSES)[number]) &&
    typeof data.title === "string" &&
    typeof data.contentHash === "string" &&
    HASH_PATTERN.test(data.contentHash) &&
    typeof withheld === "boolean" &&
    // Content is present exactly when it is not withheld.
    (withheld ? data.content === null : typeof data.content === "string") &&
    Array.isArray(data.lineage) &&
    data.lineage.length > 0 &&
    data.lineage.every(isLineageEntry)
  );
}

/**
 * Reads one stored report through the web's same-origin route, which forwards to
 * `GET /api/runs/{runId}/reports/{reportId}`. A response that is not a valid report becomes an
 * `invalid_json` failure.
 */
export async function getReport(
  runId: string,
  reportId: string,
  options?: FetchJsonOptions,
): Promise<FetchJsonResult<ReportView>> {
  const result = await fetchJson<unknown>(
    `/api/runs/${encodeURIComponent(runId)}/reports/${encodeURIComponent(reportId)}`,
    options,
  );
  if (!result.ok) return result;
  if (isReportView(result.data)) return { ...result, data: result.data };
  return {
    ok: false,
    error: { kind: "invalid_json", status: result.status },
    durationMs: result.durationMs,
    requestId: result.requestId,
  };
}
