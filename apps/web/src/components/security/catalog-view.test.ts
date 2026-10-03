import { readFileSync } from "node:fs";
import { createRequire } from "node:module";

import type { CatalogStatus } from "@workspace/contracts";
import { describe, expect, it } from "vitest";

import { controlRows, describeReload, isReloadPending, shortDigest } from "./catalog-view";

// The contract fixtures are the shapes the gateway really serves (generated from Go's handler).
function fixture(name: string): CatalogStatus {
  const path = createRequire(import.meta.url).resolve(`@workspace/contracts/fixtures/${name}`);
  return JSON.parse(readFileSync(path, "utf8")) as CatalogStatus;
}

const active = fixture("catalog-status.active.json");
const rejected = fixture("catalog-status.rejected-request.json");

describe("describeReload", () => {
  it("says a change is in effect when the active revision is the requested one", () => {
    expect(describeReload(active)).toEqual({ kind: "in_effect", activeRevisionId: 1 });
    expect(isReloadPending(active)).toBe(false);
  });

  it("reports a rejection with the active revision still in force", () => {
    const state = describeReload(rejected);
    expect(state).toMatchObject({ kind: "rejected", activeRevisionId: 1 });
    expect(state.kind === "rejected" && state.rejection).toEqual({
      code: "catalog_invalid",
      message: "The requested revision failed gateway validation.",
      revisionId: 2,
      stage: "gateway_validation",
    });
  });

  it("reports a requested revision that is not active yet as pending", () => {
    const pending: CatalogStatus = { ...active, requestedRevisionId: 3, lastError: null };
    expect(describeReload(pending)).toEqual({
      kind: "pending",
      activeRevisionId: 1,
      requestedRevisionId: 3,
    });
    expect(isReloadPending(pending)).toBe(true);
  });

  it("treats a rejection as the answer even when nothing newer is requested", () => {
    const importRejected: CatalogStatus = { ...active, lastError: rejected.lastError };
    expect(describeReload(importRejected).kind).toBe("rejected");
  });

  it("says nothing is enforceable before the first activation, keeping any rejection", () => {
    const none: CatalogStatus = {
      ...active,
      activeRevisionId: null,
      validatedRevisionId: null,
      lastError: rejected.lastError,
      controls: [],
    };
    expect(describeReload(none)).toMatchObject({
      kind: "none_active",
      rejection: { code: "catalog_invalid" },
    });
    expect(describeReload({ ...none, lastError: null })).toEqual({
      kind: "none_active",
      rejection: null,
    });
  });
});

describe("controlRows", () => {
  it("lists all three controls with their real settings", () => {
    expect(controlRows(active).map((row) => row.controlId)).toEqual([
      "secret_pattern",
      "semantic_injection",
      "signature_match",
    ]);
    const semantic = controlRows(active).find((row) => row.controlId === "semantic_injection");
    expect(semantic).toMatchObject({ enabled: true, response: "block", threshold: "0.75" });
    expect(semantic?.boundaries).toEqual(["model_input", "tool_result", "action_proposal"]);
    const secrets = controlRows(active).find((row) => row.controlId === "secret_pattern");
    expect(secrets).toMatchObject({ response: "redact", threshold: "none" });
  });

  it("explains an enabled control with no mode as the feed rules' own responses", () => {
    expect(controlRows(active).find((row) => row.controlId === "signature_match")?.response).toBe(
      "set by each feed rule",
    );
  });

  it("shows a disabled control as disabled, applied nowhere", () => {
    const disabled: CatalogStatus = {
      ...active,
      controls: active.controls.map((control) =>
        control.controlId === "semantic_injection"
          ? { ...control, enabled: false, mode: null, threshold: null }
          : control,
      ),
    };
    const row = controlRows(disabled).find(
      (candidate) => candidate.controlId === "semantic_injection",
    );
    expect(row).toMatchObject({
      enabled: false,
      response: "not applied",
      threshold: "none",
      boundaries: [],
    });
  });

  it("has no rows before the first activation", () => {
    expect(controlRows({ ...active, activeRevisionId: null, controls: [] })).toEqual([]);
  });
});

describe("shortDigest", () => {
  it("shortens a digest and says none for a missing one", () => {
    expect(shortDigest(active.policyDigest)).toBe("df00c9d60645…");
    expect(shortDigest(null)).toBe("none");
  });
});
