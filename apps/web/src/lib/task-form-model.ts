// The task form's pure logic (WEB-05): what the form offers, what the operator may combine, and the
// start-run request built from the choices. The form offers only what the server returned, never a
// default of its own, and the request holds only start-run contract fields (no actor, organization or
// grant: identity comes from the verified session, never from the browser).

import type { StartRunRequest, TaskFormOptions } from "@workspace/contracts";

import type { FetchJsonError } from "./fetch-json";

/**
 * An offered invoice. `vendorId` is the vendor the invoice belongs to; the options contract carries it so
 * the form can keep a selection to one vendor (admission rejects a set that mixes vendors). It is optional
 * until every producer sends it; an invoice without one is offered ungrouped.
 */
export type OfferedInvoice = TaskFormOptions["invoices"][number] & { vendorId?: string };

export interface InvoiceGroup {
  /** The vendor id, or null for invoices the server did not attribute to a vendor. */
  vendorId: string | null;
  vendorName: string;
  invoices: OfferedInvoice[];
}

/** The operator's choices, kept as the strings the inputs hold. */
export interface TaskFormState {
  template: string;
  vendorId: string;
  invoiceIds: string[];
  destination: string;
  approvalRequirement: string;
  modelCalls: string;
  timeoutSeconds: string;
}

export const EMPTY_FORM_STATE: TaskFormState = {
  template: "",
  vendorId: "",
  invoiceIds: [],
  destination: "",
  approvalRequirement: "",
  modelCalls: "",
  timeoutSeconds: "",
};

const soleOffered = (offered: { id: string }[]): string =>
  offered.length === 1 ? (offered[0]?.id ?? "") : "";

/**
 * The starting choices: a field is preselected only when the server offered exactly one value for it,
 * so there is nothing else to choose. Everything else, including the limits and the invoices, starts
 * empty; the limits are then the server's own defaults and the options only state the ceiling.
 */
export function initialFormState(options: TaskFormOptions): TaskFormState {
  return {
    ...EMPTY_FORM_STATE,
    template: soleOffered(options.templates),
    destination: soleOffered(options.destinations),
    approvalRequirement: soleOffered(options.approvalRequirements),
  };
}

/** Invoices grouped by vendor in the order the server listed the vendors, then any unlisted vendor. */
export function groupInvoicesByVendor(
  invoices: OfferedInvoice[],
  vendors: { id: string; name: string }[],
): InvoiceGroup[] {
  const groups: InvoiceGroup[] = [];
  const byVendor = new Map<string, InvoiceGroup>();
  const ungrouped: InvoiceGroup = { vendorId: null, vendorName: "Invoices", invoices: [] };

  for (const vendor of vendors) {
    const group: InvoiceGroup = { vendorId: vendor.id, vendorName: vendor.name, invoices: [] };
    byVendor.set(vendor.id, group);
  }
  for (const invoice of invoices) {
    if (invoice.vendorId === undefined) {
      ungrouped.invoices.push(invoice);
      continue;
    }
    let group = byVendor.get(invoice.vendorId);
    if (group === undefined) {
      group = { vendorId: invoice.vendorId, vendorName: invoice.vendorId, invoices: [] };
      byVendor.set(invoice.vendorId, group);
    }
    group.invoices.push(invoice);
  }
  groups.push(...[...byVendor.values()].filter((group) => group.invoices.length > 0));
  if (ungrouped.invoices.length > 0) groups.push(ungrouped);
  return groups;
}

/** The one vendor the selected invoices belong to, or null when none is selected or none is attributed. */
export function vendorOfSelection(
  invoices: OfferedInvoice[],
  selectedIds: string[],
): string | null {
  for (const invoice of invoices) {
    if (selectedIds.includes(invoice.id) && invoice.vendorId !== undefined) return invoice.vendorId;
  }
  return null;
}

/**
 * Whether an invoice cannot be added to the current selection because it belongs to another vendor.
 * Admission rejects a set across vendors, so the form does not let one be built.
 */
export function isInvoiceOutsideSelectedVendor(
  invoice: OfferedInvoice,
  invoices: OfferedInvoice[],
  selectedIds: string[],
): boolean {
  const selectedVendor = vendorOfSelection(invoices, selectedIds);
  return (
    selectedVendor !== null &&
    invoice.vendorId !== undefined &&
    invoice.vendorId !== selectedVendor &&
    !selectedIds.includes(invoice.id)
  );
}

const isPositiveWholeNumber = (text: string): boolean => /^[1-9][0-9]*$/.test(text.trim());

/**
 * The first reason the choices cannot be submitted, or null. It checks that every choice is one the
 * server offered and that a limit is a whole number; it does not compare a limit with the ceiling,
 * because a request above it is for admission to reject and explain (WEB-11).
 */
export function firstProblemWithChoices(
  state: TaskFormState,
  options: TaskFormOptions,
): string | null {
  const offers = (offered: { id: string }[], id: string) => offered.some((item) => item.id === id);
  if (!offers(options.templates, state.template)) return "Choose a task template.";
  if (state.vendorId !== "" && !offers(options.vendors, state.vendorId)) {
    return "Choose a vendor from the list, or leave it empty.";
  }
  if (state.invoiceIds.length === 0) return "Select at least one invoice.";
  const invoices = options.invoices as OfferedInvoice[];
  if (!state.invoiceIds.every((invoiceId) => offers(invoices, invoiceId))) {
    return "Select only invoices from the list.";
  }
  const vendorIds = new Set(
    invoices
      .filter((invoice) => state.invoiceIds.includes(invoice.id))
      .map((invoice) => invoice.vendorId)
      .filter((vendorId) => vendorId !== undefined),
  );
  if (vendorIds.size > 1) return "Select invoices from one vendor only.";
  if (!offers(options.destinations, state.destination)) return "Choose a report destination.";
  if (
    state.approvalRequirement !== "" &&
    !offers(options.approvalRequirements, state.approvalRequirement)
  ) {
    return "Choose an approval requirement from the list, or leave it empty.";
  }
  for (const [text, label] of [
    [state.modelCalls, "model calls"],
    [state.timeoutSeconds, "timeout"],
  ] as const) {
    if (text.trim() !== "" && !isPositiveWholeNumber(text)) {
      return `The ${label} limit must be a whole number of at least 1, or empty.`;
    }
  }
  return null;
}

/** The start-run request: only contract fields, and only the optional ones the operator filled in. */
export function buildStartRunRequest(state: TaskFormState): StartRunRequest {
  const request: StartRunRequest = {
    template: state.template,
    invoiceIds: [...state.invoiceIds],
    destination: state.destination,
  };
  if (state.vendorId !== "") request.vendorId = state.vendorId;
  if (state.approvalRequirement !== "") request.approvalRequirement = state.approvalRequirement;
  const modelCalls = state.modelCalls.trim();
  const timeoutSeconds = state.timeoutSeconds.trim();
  if (modelCalls !== "" || timeoutSeconds !== "") {
    request.limits = {};
    if (modelCalls !== "") request.limits.modelCalls = Number.parseInt(modelCalls, 10);
    if (timeoutSeconds !== "") request.limits.timeoutSeconds = Number.parseInt(timeoutSeconds, 10);
  }
  return request;
}

const SAFE_CODE = /^[a-z0-9_]{1,64}$/;

/**
 * Why the options could not be read, in words an operator can act on. A 503 is the gateway saying it has
 * no active control catalog: nothing may start until one is activated, and the form says so instead of
 * offering anything.
 */
export function describeOptionsFailure(error: FetchJsonError): string {
  switch (error.kind) {
    case "network":
      return "The server could not be reached, so no task options are available.";
    case "timeout":
      return "The server did not answer in time, so no task options are available.";
    case "aborted":
      return "Loading the task options was cancelled.";
    case "invalid_json":
      return "The server's reply to the task options request was not valid, so nothing is offered.";
    case "http": {
      if (error.status === 401) return "Your session has expired. Sign in again.";
      if (error.status === 503) {
        return "No active control catalog: the gateway has no policy to enforce, so no task can start. An operator must import and activate a policy (pnpm policy:import, then pnpm catalog:activate).";
      }
      const code = (error.body as { error?: { code?: unknown } } | null | undefined)?.error?.code;
      const codeNote = typeof code === "string" && SAFE_CODE.test(code) ? `, ${code}` : "";
      return `The server could not provide task options (HTTP ${error.status}${codeNote}), so nothing is offered.`;
    }
  }
}

/** A reason the form cannot be used although the options loaded: a list it needs came back empty. */
export function describeEmptyOffers(options: TaskFormOptions): string | null {
  const missing = [
    options.templates.length === 0 ? "task templates" : null,
    options.invoices.length === 0 ? "invoices" : null,
    options.destinations.length === 0 ? "report destinations" : null,
  ].filter((name) => name !== null);
  return missing.length === 0
    ? null
    : `The server offered no ${missing.join(", no ")}, so no task can be started.`;
}

/**
 * Why starting the task failed for a reason other than an admission rejection (which the rejection
 * notice explains). A timeout is reported as unknown: the server may have admitted the task.
 */
export function describeStartFailure(error: FetchJsonError): string {
  switch (error.kind) {
    case "network":
      return "The server could not be reached; the task was not started.";
    case "timeout":
      return "The server did not answer in time, so it is not known whether the task was started.";
    case "aborted":
      return "Starting the task was cancelled.";
    case "invalid_json":
      return "The server's reply was not valid, so it is not known whether the task was started.";
    case "http": {
      if (error.status === 401) return "Your session has expired. Sign in again.";
      if (error.status === 503) {
        return "No active control catalog: the gateway has no policy to enforce, so no task can start.";
      }
      const code = (error.body as { error?: { code?: unknown } } | null | undefined)?.error?.code;
      const codeNote = typeof code === "string" && SAFE_CODE.test(code) ? `, ${code}` : "";
      return `The server refused to start the task (HTTP ${error.status}${codeNote}).`;
    }
  }
}
