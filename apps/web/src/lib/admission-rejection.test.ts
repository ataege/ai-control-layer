import { describe, expect, it } from "vitest";

import {
  admissionRejectionFromError,
  explainAdmissionRejection,
  REQUEST_FIELD_LABELS,
} from "./admission-rejection";

const httpError = (status: number, code: unknown, message: unknown = "safe message") => ({
  kind: "http" as const,
  status,
  body: { error: { code, message } },
});

describe("admissionRejectionFromError", () => {
  it("reads the code and safe message of a rejected request", () => {
    expect(
      admissionRejectionFromError(
        httpError(403, "limit_not_allowed", "limits.modelCalls exceeds the catalog limit of 12"),
      ),
    ).toEqual({
      code: "limit_not_allowed",
      message: "limits.modelCalls exceeds the catalog limit of 12",
    });
  });

  it("is not a rejection when the failure is not an admission reply", () => {
    for (const error of [
      { kind: "network" as const, message: "offline" },
      { kind: "timeout" as const, timeoutMs: 1000 },
      { kind: "aborted" as const },
      { kind: "invalid_json" as const, status: 200 },
      httpError(500, "limit_not_allowed"),
      httpError(403, "approval_required"),
      httpError(401, "unauthorized"),
      { kind: "http" as const, status: 403, body: undefined },
      { kind: "http" as const, status: 403, body: { error: "text" } },
      httpError(403, 42),
    ]) {
      expect(admissionRejectionFromError(error)).toBeNull();
    }
  });

  it("keeps a missing or empty message as null rather than inventing one", () => {
    expect(
      admissionRejectionFromError(httpError(403, "template_not_allowed", ""))?.message,
    ).toBeNull();
    expect(
      admissionRejectionFromError(httpError(403, "template_not_allowed", 7))?.message,
    ).toBeNull();
  });
});

describe("explainAdmissionRejection", () => {
  it("names the scope or limit to change for each admission code", () => {
    expect(explainAdmissionRejection("limit_not_allowed")?.fieldsToChange).toEqual(["limits"]);
    expect(explainAdmissionRejection("destination_not_allowed")?.fieldsToChange).toEqual([
      "destination",
    ]);
    expect(explainAdmissionRejection("template_not_allowed")?.fieldsToChange).toEqual(["template"]);
    expect(explainAdmissionRejection("resource_out_of_scope")?.fieldsToChange).toEqual([
      "invoiceIds",
      "vendorId",
    ]);
  });

  it("only names fields the form has, and explains nothing for a non-admission code", () => {
    for (const code of [
      "resource_out_of_scope",
      "destination_not_allowed",
      "template_not_allowed",
      "limit_not_allowed",
      "invalid_arguments",
    ]) {
      for (const field of explainAdmissionRejection(code)?.fieldsToChange ?? []) {
        expect(REQUEST_FIELD_LABELS[field]).toBeTruthy();
      }
    }
    expect(explainAdmissionRejection("approval_required")).toBeNull();
    expect(explainAdmissionRejection("constructor")).toBeNull();
    expect(explainAdmissionRejection("")).toBeNull();
  });
});
