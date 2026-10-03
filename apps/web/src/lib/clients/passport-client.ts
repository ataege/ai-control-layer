import type { Passport } from "@workspace/contracts";

import { fetchJson, type FetchJsonOptions, type FetchJsonResult } from "../fetch-json";

const stringArrayFields = [
  "tools",
  "invoiceIds",
  "vendorIds",
  "reportTemplates",
  "projectionRules",
  "recipientReferences",
  "allowedModels",
  "approvalRequiredTools",
] as const;

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

/** Runtime check of the parts the panel reads; the contract type alone is not validated at runtime. */
export function isPassport(data: unknown): data is Passport {
  if (typeof data !== "object" || data === null) return false;
  const candidate = data as Record<string, unknown>;
  const scope = candidate.scope as Record<string, unknown> | null | undefined;
  const limits = candidate.limits as Record<string, unknown> | null | undefined;
  return (
    typeof candidate.passportId === "string" &&
    typeof candidate.expiresAt === "string" &&
    typeof candidate.admissionCatalogRevisionId === "number" &&
    typeof scope === "object" &&
    scope !== null &&
    stringArrayFields.every((field) => isStringArray(scope[field])) &&
    typeof scope.internalNoteReadable === "boolean" &&
    typeof limits === "object" &&
    limits !== null &&
    typeof limits.callsTotal === "number"
  );
}

/**
 * Reads the Task Passport of one run through the web app's own proxy route. A body that is not a
 * passport is reported as invalid JSON rather than rendered.
 */
export async function fetchPassport(
  runId: string,
  options?: FetchJsonOptions,
): Promise<FetchJsonResult<Passport>> {
  const result = await fetchJson<unknown>(
    `/api/runs/${encodeURIComponent(runId)}/passport`,
    options,
  );
  if (!result.ok) return result;
  if (!isPassport(result.data)) {
    return {
      ok: false,
      error: { kind: "invalid_json", status: result.status },
      durationMs: result.durationMs,
      requestId: result.requestId,
    };
  }
  return { ...result, data: result.data };
}
