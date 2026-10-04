import { describe, expect, it, vi, afterEach } from "vitest";
import type { ReasonCode } from "@workspace/contracts";
import reasonCodeContract from "@workspace/contracts/schemas/reason-code.schema.json";
import { classifyFailure } from "./errors/failure";
import { REASON_FAILURES } from "./errors/reason-failures";
import { ProductClient, getSafeMessage } from "./product-client";
import { postJson } from "./fetch-json";

vi.mock("./fetch-json", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./fetch-json")>();
  return {
    ...actual,
    fetchJson: vi.fn(),
    postJson: vi.fn(),
  };
});

describe("ProductClient", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  describe("startRun", () => {
    it("passes guard with valid response", async () => {
      vi.mocked(postJson).mockResolvedValueOnce({
        ok: true,
        status: 200,
        data: { runId: "r-123", passportId: "p-456" },
        durationMs: 10,
      });

      const result = await ProductClient.startRun({
        template: "test",
        invoiceIds: ["inv-1"],
        destination: "dest-1",
      });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.data.runId).toBe("r-123");
      }
      expect(postJson).toHaveBeenCalledTimes(1);
    });

    it("fails guard with missing fields", async () => {
      vi.mocked(postJson).mockResolvedValueOnce({
        ok: true,
        status: 200,
        data: { runId: "r-123" }, // missing passportId
        durationMs: 10,
      });

      const result = await ProductClient.startRun({
        template: "test",
        invoiceIds: ["inv-1"],
        destination: "dest-1",
      });

      expect(result).toMatchObject({
        ok: false,
        error: { kind: "invalid_json", status: 200 },
      });
    });

    it("does not retry on timeout", async () => {
      vi.mocked(postJson).mockResolvedValueOnce({
        ok: false,
        error: { kind: "timeout", timeoutMs: 10000 },
        durationMs: 10000,
      });

      const result = await ProductClient.startRun({
        template: "test",
        invoiceIds: ["inv-1"],
        destination: "dest-1",
      });

      expect(result).toMatchObject({
        ok: false,
        error: { kind: "timeout" },
      });
      // Ensure only called once
      expect(postJson).toHaveBeenCalledTimes(1);
    });
  });

  describe("getSafeMessage", () => {
    it("maps a known reason code to the typed mapping's fixed text", () => {
      expect(getSafeMessage("approval_required")).toBe(REASON_FAILURES.approval_required.message);
    });

    it("returns the generic message for an unknown code, string or error body", () => {
      const generic = "The request failed in a way this page does not recognize.";
      expect(getSafeMessage("unknown_random_code")).toBe(generic);
      expect(
        getSafeMessage({
          kind: "http",
          status: 418,
          body: { error: { code: "unknown_random_code", message: "server words" } },
        }),
      ).toBe(generic);
    });

    it("never shows a server message", () => {
      const message = getSafeMessage({
        kind: "http",
        status: 400,
        body: { error: { code: "limit_not_allowed", message: "server words, secret detail" } },
      });
      expect(message).toBe(REASON_FAILURES.limit_not_allowed.message);
      expect(message).not.toContain("secret");
    });
  });

  describe("the reason vocabulary", () => {
    it("has the fixed text of every X-13 code in the contract, and nothing else", () => {
      const contractCodes: string[] = reasonCodeContract.enum;
      expect(Object.keys(REASON_FAILURES).sort()).toEqual([...contractCodes].sort());
      for (const code of contractCodes) {
        expect(getSafeMessage(code)).toBe(REASON_FAILURES[code as ReasonCode].message);
      }
    });
  });

  describe("a command that gets no answer", () => {
    it("is reported unconfirmed and is sent only once", async () => {
      vi.mocked(postJson).mockResolvedValueOnce({
        ok: false,
        error: { kind: "timeout", timeoutMs: 10000 },
        durationMs: 10000,
      });
      const result = await ProductClient.startRun({
        template: "test",
        invoiceIds: ["inv-1"],
        destination: "dest-1",
      });
      expect(result.ok).toBe(false);
      if (!result.ok) {
        const failure = classifyFailure(result.error, { command: true });
        expect(failure.unconfirmed).toBe(true);
        // Submitting a command again before checking could repeat it, so no retry is offered.
        expect(failure.action).toBe("none");
      }
      expect(postJson).toHaveBeenCalledTimes(1);
    });
  });
});
