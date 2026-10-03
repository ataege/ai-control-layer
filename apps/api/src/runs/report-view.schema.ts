import { z } from "zod";
import contract from "@workspace/contracts/schemas/report-view.schema.json" with { type: "json" };
import type { ReportView } from "@workspace/contracts";

const { allOf: conditions, ...shape } = contract;
void conditions;
// Apply the shared conditional content/projection rules without deriving any classification.
export const ReportViewSchema = z
  .fromJSONSchema(shape as Parameters<typeof z.fromJSONSchema>[0])
  .superRefine((value, context) => {
    const report = value as ReportView;
    if (
      report.contentWithheld
        ? report.content !== null || report.classification !== "internal_only"
        : report.content === null
    ) {
      context.addIssue({ code: "custom", message: "Invalid withheld report content" });
    }
    const vendor = report.template === "vendor_reconciliation_v1";
    if (
      vendor
        ? report.projectionRule !== "vendor_invoice_fields_v1" ||
          report.projectionRuleVersion === null
        : report.projectionRule !== null || report.projectionRuleVersion !== null
    ) {
      context.addIssue({ code: "custom", message: "Invalid report projection" });
    }
  });
