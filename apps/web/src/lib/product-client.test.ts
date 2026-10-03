import { describe, expect, it, vi, afterEach } from "vitest";
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
    it("maps known reason code", () => {
      expect(getSafeMessage("approval_required")).toBe("This action requires explicit approval.");
    });
    
    it("returns generic message for unknown code", () => {
      expect(getSafeMessage("unknown_random_code")).toBe("An unknown error occurred.");
    });
  });
});
