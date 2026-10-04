import { ServiceUnavailableException } from "@nestjs/common";
import type { PolicyStatusResponse } from "@workspace/contracts";
import { z } from "zod";
import contract from "@workspace/contracts/schemas/policy-status-response.schema.json" with { type: "json" };
import type { ControlCatalogPointer } from "./control-catalog-pointer.entity.js";

const statusSchema = z.fromJSONSchema(contract as Parameters<typeof z.fromJSONSchema>[0]);
const issueSchema = z.object({ path: z.string(), message: z.string() });

export function policyStatus(pointer: ControlCatalogPointer): PolicyStatusResponse {
  let lastError: PolicyStatusResponse["lastError"] = null;
  const record = pointer.lastError;
  if (record !== null) {
    if (record.stage === "gateway_validation") {
      const revision = record.revision_id;
      // JSON numeric IDs must remain exact; never silently round an int64.
      if (
        typeof revision !== "number" ||
        !Number.isSafeInteger(revision) ||
        revision <= 0 ||
        typeof record.code !== "string" ||
        typeof record.message !== "string"
      )
        throw new ServiceUnavailableException("Invalid stored policy error");
      lastError = {
        code: record.code,
        message: record.message,
        revisionId: String(revision),
        stage: "gateway_validation",
      };
    } else {
      const issues = z.array(issueSchema).min(1).safeParse(record.issues);
      if (
        record.reason !== "policy_reload_rejected" ||
        (record.stage !== undefined && record.stage !== "import_validation") ||
        !issues.success
      )
        throw new ServiceUnavailableException("Invalid stored policy error");
      lastError = {
        code: "policy_reload_rejected",
        message: issues.data.map((issue) => `${issue.path}: ${issue.message}`).join("; "),
        revisionId: null,
        stage: "import_validation",
      };
    }
  }
  const parsed = statusSchema.safeParse({
    requestedRevisionId: pointer.requestedRevisionId,
    validatedRevisionId: pointer.validatedRevisionId,
    activeRevisionId: pointer.activeRevisionId,
    activeFeedRevisionId: pointer.activeFeedRevisionId,
    lastError,
  });
  if (!parsed.success) throw new ServiceUnavailableException("Invalid stored policy status");
  return parsed.data as PolicyStatusResponse;
}
