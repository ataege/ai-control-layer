import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import type { TaskFormOptions } from "@workspace/contracts";
import { describe, expect, it, vi } from "vitest";

import { classifyFailure } from "@/lib/errors/failure";
import { TaskFormView } from "./task-form";
import type { TaskFormState } from "@/lib/task-form-model";

// The form reads the router only in the stateful container; the view under test never calls it.
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: () => {} }),
  usePathname: () => "/tasks/new",
}));

const options: TaskFormOptions = {
  templates: [{ id: "reconcile_atlas_v1", name: "Reconcile Atlas invoices" }],
  vendors: [
    { id: "vendor_Atlas", name: "Atlas" },
    { id: "vendor_Borealis", name: "Borealis" },
  ],
  invoices: [
    { id: "invoice_A01", number: "INV104", date: "2026-09-01", amount: 125000 },
    { id: "invoice_B01", number: "INV220", date: "2026-09-02", amount: 80000 },
  ].map((invoice, index) => ({
    ...invoice,
    vendorId: index === 0 ? "vendor_Atlas" : "vendor_Borealis",
  })),
  destinations: [
    { id: "vendor_Atlas", name: "Atlas" },
    { id: "vendor_Borealis", name: "Borealis" },
  ],
  approvalRequirements: [{ id: "review_queue_report", description: "A reviewer approves" }],
  limits: { maxModelCalls: 24, maxTimeoutSeconds: 900 },
};

const state: TaskFormState = {
  template: "reconcile_atlas_v1",
  vendorId: "",
  invoiceIds: ["invoice_A01"],
  destination: "vendor_Atlas",
  approvalRequirement: "review_queue_report",
  modelCalls: "999",
  timeoutSeconds: "",
};

const render = (overrides: Partial<Parameters<typeof TaskFormView>[0]> = {}) =>
  renderToStaticMarkup(
    createElement(TaskFormView, {
      options,
      state,
      onChange: () => {},
      onSubmit: () => {},
      isSubmitting: false,
      problem: null,
      failure: null,
      rejection: null,
      ...overrides,
    }),
  );

describe("TaskFormView", () => {
  it("groups the invoices by vendor and blocks the other vendor's invoices once one is selected", () => {
    const html = render();
    expect(html).toContain("Atlas");
    expect(html).toContain("Borealis");
    expect(html).toContain("INV104 · 2026-09-01 · 125000 (minor units, currency not stated)");
    // Atlas's invoice is selected, so Borealis's is not selectable; the selected one stays enabled.
    const borealisCheckbox = html.match(/<button[^>]*id="invoice-invoice_B01"[^>]*>/)?.[0] ?? "";
    const atlasCheckbox = html.match(/<button[^>]*id="invoice-invoice_A01"[^>]*>/)?.[0] ?? "";
    expect(atlasCheckbox).not.toBe("");
    expect(borealisCheckbox).toContain('disabled=""');
    expect(atlasCheckbox).not.toContain('disabled=""');
  });

  it("shows an invoice's amount as money once the invoice names its currency", () => {
    const withCurrency = {
      ...options,
      invoices: options.invoices.map((invoice) => ({ ...invoice, currency: "EUR" })),
    };
    expect(render({ options: withCurrency })).toContain(
      "INV104 · 2026-09-01 · \u20ac1,250.00 (EUR)",
    );
  });

  it("states the limits ceiling without capping the input, so an over-limit request reaches admission", () => {
    const html = render();
    expect(html).toContain("up to 24");
    expect(html).toContain("up to 900");
    expect(html).not.toMatch(/<input[^>]*max=/);
  });

  it("shows an admission rejection with its reason and the field to change, and no passport or run", () => {
    const html = render({
      rejection: { code: "limit_not_allowed" },
    });
    expect(html).toContain('data-admission-rejection="limit_not_allowed"');
    expect(html).toContain("A requested limit is above what the active policy allows.");
    expect(html).toContain("no passport was issued and no run started");
    expect(html).toContain("Submit revised request");
    expect(html).not.toMatch(/Passport (issued|id)|Run id/i);
  });

  it("keeps the operator's choices while a rejection is shown", () => {
    const html = render({
      rejection: { code: "limit_not_allowed" },
    });
    // The limit the operator typed is still in the field; nothing narrowed it.
    expect(html).toContain('value="999"');
  });

  it("shows a local problem with the choices as a plain alert, not as an admission rejection", () => {
    const html = render({ problem: "Select at least one invoice." });
    expect(html).toContain("Select at least one invoice.");
    expect(html).not.toContain("data-admission-rejection");
  });

  it("shows a failed start in its own truthful state, unconfirmed with no retry when no answer came", () => {
    const html = render({
      failure: {
        failure: classifyFailure({ kind: "timeout", timeoutMs: 1000 }, { command: true }),
        requestId: "req_42",
      },
    });
    expect(html).toContain('data-failure-kind="timeout"');
    expect(html).toContain("Outcome unconfirmed.");
    expect(html).toContain("Request ID: req_42");
    expect(html).not.toContain("Try again");
    expect(html).not.toContain("data-admission-rejection");
  });

  it("offers a retry for a failed start that is known not to have been carried out", () => {
    const html = render({
      failure: {
        failure: classifyFailure(
          { kind: "http", status: 502, body: { error: { code: "upstream_unreachable" } } },
          { command: true },
        ),
      },
    });
    expect(html).toContain('data-failure-kind="upstream_unreachable"');
    expect(html).toContain("Try again");
  });

  it("does not call onSubmit or onChange by rendering", () => {
    const onSubmit = vi.fn();
    const onChange = vi.fn();
    render({ onSubmit, onChange, rejection: { code: "limit_not_allowed" } });
    expect(onSubmit).not.toHaveBeenCalled();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("disables submitting while a submission is in flight", () => {
    expect(render({ isSubmitting: true })).toContain("Starting Task...");
  });
});
