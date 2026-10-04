import type { TaskFormOptions } from "@workspace/contracts";
import { describe, expect, it } from "vitest";

import {
  buildStartRunRequest,
  classifyOptionsFailure,
  describeEmptyOffers,
  EMPTY_FORM_STATE,
  firstProblemWithChoices,
  formatInvoiceAmount,
  groupInvoicesByVendor,
  initialFormState,
  isInvoiceOutsideSelectedVendor,
  type OfferedInvoice,
  type TaskFormState,
} from "./task-form-model";

const options: TaskFormOptions = {
  templates: [{ id: "reconcile_atlas_v1", name: "Reconcile Atlas invoices" }],
  vendors: [
    { id: "vendor_Atlas", name: "Atlas" },
    { id: "vendor_Borealis", name: "Borealis" },
  ],
  invoices: [
    {
      id: "invoice_A01",
      number: "INV104",
      date: "2026-09-01",
      amount: 125000,
      vendorId: "vendor_Atlas",
      currency: "EUR",
    },
    {
      id: "invoice_A02",
      number: "INV104",
      date: "2026-09-08",
      amount: 125000,
      vendorId: "vendor_Atlas",
      currency: "EUR",
    },
  ],
  destinations: [
    { id: "vendor_Atlas", name: "Atlas" },
    { id: "vendor_Borealis", name: "Borealis" },
  ],
  approvalRequirements: [{ id: "review_queue_report", description: "A reviewer approves" }],
  limits: { maxModelCalls: 24, maxTimeoutSeconds: 900 },
};

const vendorInvoices: OfferedInvoice[] = [
  {
    id: "invoice_A01",
    number: "INV1",
    date: "2026-09-01",
    amount: 1,
    vendorId: "vendor_Atlas",
    currency: "EUR",
  },
  {
    id: "invoice_B01",
    number: "INV2",
    date: "2026-09-02",
    amount: 2,
    vendorId: "vendor_Borealis",
    currency: "EUR",
  },
  {
    id: "invoice_A02",
    number: "INV3",
    date: "2026-09-03",
    amount: 3,
    vendorId: "vendor_Atlas",
    currency: "EUR",
  },
];

const validState: TaskFormState = {
  template: "reconcile_atlas_v1",
  vendorId: "",
  invoiceIds: ["invoice_A01"],
  destination: "vendor_Atlas",
  approvalRequirement: "",
  modelCalls: "",
  timeoutSeconds: "",
};

describe("initialFormState", () => {
  it("preselects only what the server offered exactly once", () => {
    const state = initialFormState(options);
    expect(state.template).toBe("reconcile_atlas_v1");
    expect(state.approvalRequirement).toBe("review_queue_report");
    // Two destinations and two vendors: nothing is chosen for the operator.
    expect(state.destination).toBe("");
    expect(state.vendorId).toBe("");
    expect(state.invoiceIds).toEqual([]);
    expect(state.modelCalls).toBe("");
    expect(state.timeoutSeconds).toBe("");
  });

  it("selects nothing at all when the server offered nothing", () => {
    const empty = { ...options, templates: [], destinations: [], approvalRequirements: [] };
    expect(initialFormState(empty)).toEqual(EMPTY_FORM_STATE);
  });
});

describe("groupInvoicesByVendor", () => {
  it("groups by vendor in the server's vendor order and drops empty vendors", () => {
    const groups = groupInvoicesByVendor(vendorInvoices, options.vendors);
    expect(groups.map((group) => [group.vendorName, group.invoices.map((i) => i.id)])).toEqual([
      ["Atlas", ["invoice_A01", "invoice_A02"]],
      ["Borealis", ["invoice_B01"]],
    ]);
    expect(
      groupInvoicesByVendor([vendorInvoices[0]!], options.vendors).map((g) => g.vendorName),
    ).toEqual(["Atlas"]);
  });
});

describe("isInvoiceOutsideSelectedVendor", () => {
  it("blocks an invoice of another vendor once one vendor's invoice is selected", () => {
    const [atlasOne, borealis, atlasTwo] = vendorInvoices as [
      OfferedInvoice,
      OfferedInvoice,
      OfferedInvoice,
    ];
    expect(isInvoiceOutsideSelectedVendor(borealis, vendorInvoices, ["invoice_A01"])).toBe(true);
    expect(isInvoiceOutsideSelectedVendor(atlasTwo, vendorInvoices, ["invoice_A01"])).toBe(false);
    // The selected invoice is never "outside", so it can still be unchecked.
    expect(isInvoiceOutsideSelectedVendor(atlasOne, vendorInvoices, ["invoice_A01"])).toBe(false);
    // Nothing selected: everything is available.
    expect(isInvoiceOutsideSelectedVendor(borealis, vendorInvoices, [])).toBe(false);
  });
});

describe("firstProblemWithChoices", () => {
  it("accepts a valid selection", () => {
    expect(firstProblemWithChoices(validState, options)).toBeNull();
  });

  it("rejects a choice the server did not offer", () => {
    expect(
      firstProblemWithChoices({ ...validState, template: "reconcile_other" }, options),
    ).toMatch(/template/);
    expect(firstProblemWithChoices({ ...validState, destination: "attacker" }, options)).toMatch(
      /destination/,
    );
    expect(firstProblemWithChoices({ ...validState, invoiceIds: ["invoice_X"] }, options)).toMatch(
      /from the list/,
    );
    expect(firstProblemWithChoices({ ...validState, vendorId: "vendor_X" }, options)).toMatch(
      /vendor/,
    );
    expect(
      firstProblemWithChoices({ ...validState, approvalRequirement: "skip_review" }, options),
    ).toMatch(/approval/);
  });

  it("requires at least one invoice and a single vendor", () => {
    expect(firstProblemWithChoices({ ...validState, invoiceIds: [] }, options)).toMatch(
      /at least one invoice/,
    );
    const withVendors = { ...options, invoices: vendorInvoices };
    expect(
      firstProblemWithChoices(
        { ...validState, invoiceIds: ["invoice_A01", "invoice_B01"] },
        withVendors,
      ),
    ).toMatch(/one vendor/);
  });

  it("requires whole-number limits but leaves a limit above the ceiling to admission", () => {
    expect(firstProblemWithChoices({ ...validState, modelCalls: "0" }, options)).toMatch(
      /model calls/,
    );
    expect(firstProblemWithChoices({ ...validState, timeoutSeconds: "1.5" }, options)).toMatch(
      /timeout/,
    );
    expect(firstProblemWithChoices({ ...validState, modelCalls: "abc" }, options)).not.toBeNull();
    // Over the 24-call ceiling: valid to submit, so admission rejects it and the notice explains it.
    expect(firstProblemWithChoices({ ...validState, modelCalls: "999" }, options)).toBeNull();
  });
});

describe("buildStartRunRequest", () => {
  it("holds only start-run contract fields, never an actor, organization or grant", () => {
    const request = buildStartRunRequest({
      ...validState,
      vendorId: "vendor_Atlas",
      approvalRequirement: "review_queue_report",
      modelCalls: "12",
      timeoutSeconds: "600",
    });
    expect(request).toEqual({
      template: "reconcile_atlas_v1",
      vendorId: "vendor_Atlas",
      invoiceIds: ["invoice_A01"],
      destination: "vendor_Atlas",
      approvalRequirement: "review_queue_report",
      limits: { modelCalls: 12, timeoutSeconds: 600 },
    });
    for (const forbidden of ["actorId", "organizationId", "passport", "grant", "userId"]) {
      expect(request).not.toHaveProperty(forbidden);
    }
  });

  it("omits every optional field the operator left empty", () => {
    expect(buildStartRunRequest(validState)).toEqual({
      template: "reconcile_atlas_v1",
      invoiceIds: ["invoice_A01"],
      destination: "vendor_Atlas",
    });
    expect(buildStartRunRequest({ ...validState, modelCalls: "5" }).limits).toEqual({
      modelCalls: 5,
    });
  });

  it("copies the invoice list so a later edit cannot change the built request", () => {
    const state = { ...validState, invoiceIds: ["invoice_A01"] };
    const request = buildStartRunRequest(state);
    state.invoiceIds.push("invoice_A02");
    expect(request.invoiceIds).toEqual(["invoice_A01"]);
  });
});

describe("classifyOptionsFailure", () => {
  it("says honestly that a 503 means no active control catalog, as a read that offers a retry", () => {
    const failure = classifyOptionsFailure({ kind: "http", status: 503, body: undefined });
    expect(failure.title).toBe("No active control catalog");
    expect(failure.description).toContain("no task can start");
    expect(failure.description).toContain("pnpm catalog:activate");
    expect(failure.unconfirmed).toBe(false);
    expect(failure.action).toBe("retry");
  });

  it("uses the shared classification for everything else, with fixed words", () => {
    expect(classifyOptionsFailure({ kind: "http", status: 401, body: undefined }).kind).toBe(
      "session_expired",
    );
    expect(classifyOptionsFailure({ kind: "network", message: "x" }).kind).toBe("offline");
    expect(classifyOptionsFailure({ kind: "timeout", timeoutMs: 1 }).kind).toBe("timeout");
    expect(
      classifyOptionsFailure({
        kind: "http",
        status: 400,
        body: { error: { code: "bad_request", message: "Invalid record identifier" } },
      }).kind,
    ).toBe("bad_request");
  });

  it("never echoes a server-supplied message", () => {
    const failure = classifyOptionsFailure({
      kind: "http",
      status: 500,
      body: { error: { code: "internal_error", message: "stack trace secret" } },
    });
    expect(JSON.stringify(failure)).not.toContain("secret");
  });

  it("does not turn a different 5xx into the catalog message", () => {
    expect(classifyOptionsFailure({ kind: "http", status: 500, body: undefined }).title).not.toBe(
      "No active control catalog",
    );
  });
});

describe("describeEmptyOffers", () => {
  it("names what the server did not offer, and is null when everything needed is offered", () => {
    expect(describeEmptyOffers(options)).toBeNull();
    expect(describeEmptyOffers({ ...options, invoices: [], destinations: [] })).toBe(
      "The server offered no invoices, no report destinations, so no task can be started.",
    );
  });
});

describe("formatInvoiceAmount", () => {
  it("shows money in the invoice's own currency, with that currency's minor-unit digits", () => {
    expect(formatInvoiceAmount(125000, "EUR")).toBe("€1,250.00 (EUR)");
    expect(formatInvoiceAmount(125000, "USD")).toBe("$1,250.00 (USD)");
    // A zero-decimal currency has no minor units to divide out.
    expect(formatInvoiceAmount(1250, "JPY")).toBe("¥1,250 (JPY)");
  });

  it("shows the raw figure with its code when the platform cannot format the code", () => {
    expect(formatInvoiceAmount(125000, "euro")).toBe("125000 (minor units of euro)");
    expect(formatInvoiceAmount(125000, "")).toBe("125000 (minor units of )");
  });
});
