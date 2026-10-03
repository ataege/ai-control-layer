import { getSafeMessage } from "@/lib/product-client";
import type { FetchJsonError } from "@/lib/fetch-json";

export interface ReportFailure {
  title: string;
  description: string;
}

/**
 * The words for a report that could not be shown. A report outside its registered template (or an
 * incomplete one) is an error, never free text: the interface renders only stored reports.
 */
export function describeReportFailure(error: FetchJsonError): ReportFailure {
  if (error.kind === "http" && error.status === 404) {
    return {
      title: "Report not found",
      description:
        "No stored report with this identifier exists in this run for your organization.",
    };
  }
  if (error.kind === "invalid_json") {
    return {
      title: "Report cannot be shown",
      description:
        "The stored report does not match its registered template or is incomplete, so it is not shown.",
    };
  }
  return { title: "Report unavailable", description: getSafeMessage(error) };
}
