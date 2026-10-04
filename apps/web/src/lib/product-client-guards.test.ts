// The guards of the typed client accept a response only in the exact shape of its frozen contract
// (WEB-03): each contract fixture passes, and a missing field, a wrong type or an extra field is
// refused, so a response that does not match is never shown as a successful state.
import optionsFixture from "@workspace/contracts/fixtures/task-form-options.atlas.json";
import startRunFixture from "@workspace/contracts/fixtures/start-run-response.created.json";
import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchJson, postJson } from "./fetch-json";
import {
  isOperatorProfile,
  isSignInResponse,
  isStartRunResponse,
  isTaskFormOptions,
  ProductClient,
} from "./product-client";

vi.mock("./fetch-json", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./fetch-json")>();
  return { ...actual, fetchJson: vi.fn(), postJson: vi.fn() };
});

afterEach(() => vi.clearAllMocks());

/** A copy of the record without one key, to test that a missing field is refused. */
function withoutKey(record: Record<string, unknown>, key: string): Record<string, unknown> {
  return Object.fromEntries(Object.entries(record).filter(([name]) => name !== key));
}

const operatorProfile = {
  id: "7b9f1c1e-0000-4000-8000-000000000001",
  email: "demo-operator@example.com",
  name: "Development Demonstration Operator",
  organizationId: "7b9f1c1e-0000-4000-8000-000000000002",
  roles: ["operator", "reviewer"],
};

describe("the start-run answer", () => {
  it("accepts the contract fixture and nothing else", () => {
    expect(isStartRunResponse(startRunFixture)).toBe(true);
    expect(isStartRunResponse({ runId: "r" })).toBe(false);
    expect(isStartRunResponse({ ...startRunFixture, passportId: "" })).toBe(false);
    expect(isStartRunResponse({ ...startRunFixture, passportId: 7 })).toBe(false);
    expect(isStartRunResponse({ ...startRunFixture, extra: "field" })).toBe(false);
    expect(isStartRunResponse(null)).toBe(false);
    expect(isStartRunResponse([])).toBe(false);
  });
});

describe("the task form options", () => {
  it("accepts the contract fixture", () => {
    expect(isTaskFormOptions(optionsFixture)).toBe(true);
  });

  it("refuses a missing list, a missing limit and a wrong type", () => {
    const withoutLimits = withoutKey(optionsFixture, "limits");
    expect(isTaskFormOptions(withoutLimits)).toBe(false);
    expect(isTaskFormOptions({ ...optionsFixture, templates: undefined })).toBe(false);
    expect(
      isTaskFormOptions({ ...optionsFixture, limits: { maxModelCalls: 0, maxTimeoutSeconds: 9 } }),
    ).toBe(false);
    expect(
      isTaskFormOptions({
        ...optionsFixture,
        limits: { maxModelCalls: "24", maxTimeoutSeconds: 9 },
      }),
    ).toBe(false);
    expect(isTaskFormOptions({ templates: [], vendors: [] })).toBe(false);
  });

  it("refuses an invoice without its vendor or currency, or with a malformed one", () => {
    const [invoice] = optionsFixture.invoices as Record<string, unknown>[];
    const withInvoice = (changed: Record<string, unknown>) => ({
      ...optionsFixture,
      invoices: [{ ...invoice, ...changed }],
    });
    expect(isTaskFormOptions(withInvoice({}))).toBe(true);
    expect(isTaskFormOptions(withInvoice({ vendorId: undefined }))).toBe(false);
    expect(isTaskFormOptions(withInvoice({ currency: undefined }))).toBe(false);
    expect(isTaskFormOptions(withInvoice({ currency: "euro" }))).toBe(false);
    expect(isTaskFormOptions(withInvoice({ amount: 12.5 }))).toBe(false);
    expect(isTaskFormOptions(withInvoice({ date: "01/09/2026" }))).toBe(false);
    expect(isTaskFormOptions(withInvoice({ extra: true }))).toBe(false);
  });
});

describe("the operator profile and the sign-in answer", () => {
  it("accepts the five-field profile, with or without the server's demonstration flag", () => {
    expect(isOperatorProfile(operatorProfile)).toBe(true);
    expect(isOperatorProfile({ ...operatorProfile, developmentDemonstration: true })).toBe(true);
  });

  it("refuses a profile with a missing, mistyped or extra field", () => {
    const withoutOrganization = withoutKey(operatorProfile, "organizationId");
    expect(isOperatorProfile(withoutOrganization)).toBe(false);
    expect(isOperatorProfile({ ...operatorProfile, roles: "operator" })).toBe(false);
    expect(isOperatorProfile({ ...operatorProfile, roles: [1] })).toBe(false);
    expect(isOperatorProfile({ ...operatorProfile, developmentDemonstration: "yes" })).toBe(false);
    expect(isOperatorProfile({ ...operatorProfile, passwordHash: "x" })).toBe(false);
    expect(isOperatorProfile({ id: "x", name: "y" })).toBe(false);
  });

  it("accepts only the message of a sign-in answer", () => {
    expect(isSignInResponse({ message: "Signed in successfully" })).toBe(true);
    expect(isSignInResponse({})).toBe(false);
    expect(isSignInResponse({ message: 1 })).toBe(false);
    expect(isSignInResponse({ message: "ok", sessionId: "secret" })).toBe(false);
  });
});

describe("the client refuses a response that does not match its contract", () => {
  it("returns invalid_json for options, a profile and a start answer that are not the contract", async () => {
    vi.mocked(fetchJson).mockResolvedValueOnce({
      ok: true,
      status: 200,
      data: { templates: [], vendors: [] },
      durationMs: 1,
    });
    expect(await ProductClient.getOptions()).toMatchObject({
      ok: false,
      error: { kind: "invalid_json" },
    });
    vi.mocked(fetchJson).mockResolvedValueOnce({
      ok: true,
      status: 200,
      data: { id: "x", name: "y" },
      durationMs: 1,
    });
    expect(await ProductClient.getMe()).toMatchObject({
      ok: false,
      error: { kind: "invalid_json" },
    });
    vi.mocked(postJson).mockResolvedValueOnce({
      ok: true,
      status: 201,
      data: { runId: "r" },
      durationMs: 1,
    });
    expect(
      await ProductClient.startRun({ template: "t", invoiceIds: ["i"], destination: "d" }),
    ).toMatchObject({ ok: false, error: { kind: "invalid_json" } });
  });

  it("treats an empty successful answer to a command as no data, and a guarded one as invalid", async () => {
    vi.mocked(postJson).mockResolvedValueOnce({
      ok: true,
      status: 204,
      data: undefined,
      durationMs: 1,
    });
    expect(await ProductClient.signOut()).toMatchObject({ ok: true, status: 204 });
    vi.mocked(postJson).mockResolvedValueOnce({
      ok: true,
      status: 204,
      data: undefined,
      durationMs: 1,
    });
    // A start answer must carry the run and passport ids: an empty body is not a started run.
    expect(
      await ProductClient.startRun({ template: "t", invoiceIds: ["i"], destination: "d" }),
    ).toMatchObject({ ok: false, error: { kind: "invalid_json" } });
  });

  it("sends the sign-out as one POST and does not repeat it after a failure", async () => {
    vi.mocked(postJson).mockResolvedValueOnce({
      ok: false,
      error: { kind: "network", message: "offline" },
      durationMs: 1,
    });
    expect(await ProductClient.signOut()).toMatchObject({ ok: false });
    expect(postJson).toHaveBeenCalledTimes(1);
    expect(postJson).toHaveBeenCalledWith("/api/auth/sign-out", {});
  });
});
