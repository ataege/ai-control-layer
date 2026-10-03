import { classifyFailure, type Failure, type FailureWords } from "@/lib/errors/failure";
import type { FetchJsonError } from "@/lib/fetch-json";

// The report page's own words for the two failures that are about the report itself; every other
// failure (offline, an expired session, an unreachable API, ...) uses the shared states (WEB-23).
// A report outside its registered template (or an incomplete one) is an error, never free text.
export const REPORT_FAILURE_WORDS: Partial<Record<"not_found" | "invalid_response", FailureWords>> =
  {
    not_found: {
      title: "Report not found",
      description:
        "No stored report with this identifier exists in this run for your organization.",
    },
    invalid_response: {
      title: "Report cannot be shown",
      description:
        "The stored report does not match its registered template or is incomplete, so it is not shown.",
    },
  };

export function describeReportFailure(error: FetchJsonError): Failure {
  return classifyFailure(error, { overrides: REPORT_FAILURE_WORDS });
}
