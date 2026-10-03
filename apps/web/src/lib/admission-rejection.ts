// Explains an admission rejection (WEB-11). Admission is Go's decision (docs/contracts, X-13): a request
// that asks for more authority than the operator holds is rejected with a stable reason code and a safe
// message. Nothing here narrows, retries or edits the request; it only names what was unavailable and
// which choice must change, so the operator can resubmit a revised request themselves.

import type { FetchJsonError } from "./fetch-json";

/** The task form's fields that a rejection can ask the operator to change. */
export type RequestField =
  "template" | "vendorId" | "invoiceIds" | "destination" | "approvalRequirement" | "limits";

export const REQUEST_FIELD_LABELS: Record<RequestField, string> = {
  template: "Task template",
  vendorId: "Vendor",
  invoiceIds: "Invoices",
  destination: "Report destination",
  approvalRequirement: "Approval requirement",
  limits: "Limits (model calls, timeout)",
};

export interface AdmissionRejectionExplanation {
  /** What was unavailable, in plain words. */
  unavailable: string;
  /** The form fields the operator must look at before resubmitting. */
  fieldsToChange: RequestField[];
  /** What a revised request has to do. */
  whatToChange: string;
}

// The reason codes admission itself can return (services/gateway/internal/admission). Any other code
// is not an admission rejection and is shown by the form's ordinary error text.
const EXPLANATIONS: Record<string, AdmissionRejectionExplanation> = {
  resource_out_of_scope: {
    unavailable: "A selected invoice or vendor is outside the authority your organization holds.",
    fieldsToChange: ["invoiceIds", "vendorId"],
    whatToChange:
      "Select only invoices available to your organization, all belonging to one vendor, and the vendor those invoices belong to.",
  },
  destination_not_allowed: {
    unavailable: "The chosen report destination is not permitted for this task.",
    fieldsToChange: ["destination"],
    whatToChange:
      "Choose the destination registered for the vendor of the selected invoices; no other destination is allowed.",
  },
  template_not_allowed: {
    unavailable: "The chosen task template is not registered for this request.",
    fieldsToChange: ["template"],
    whatToChange: "Choose one of the task templates the form offers.",
  },
  limit_not_allowed: {
    unavailable: "A requested limit is higher than the active policy permits.",
    fieldsToChange: ["limits"],
    whatToChange:
      "Lower the model-call limit or the timeout to within the policy ceiling shown on the form.",
  },
  invalid_arguments: {
    unavailable: "The request is not valid as submitted.",
    fieldsToChange: ["template", "invoiceIds", "destination", "approvalRequirement", "limits"],
    whatToChange: "Correct the fields named in the message below and submit again.",
  },
};

export interface AdmissionRejection {
  /** The stable X-13 reason code. */
  code: string;
  /** The server's safe message, or null when the response carried none. */
  message: string | null;
}

export function isAdmissionRejectionCode(code: string): boolean {
  return Object.prototype.hasOwnProperty.call(EXPLANATIONS, code);
}

/** The explanation for an admission rejection code, or null when the code is not one admission returns. */
export function explainAdmissionRejection(code: string): AdmissionRejectionExplanation | null {
  return isAdmissionRejectionCode(code) ? (EXPLANATIONS[code] ?? null) : null;
}

/**
 * Reads an admission rejection from a failed start-run response: a client error (4xx) whose safe error
 * body carries an admission reason code. Anything else (network failure, a 5xx, another code, a body
 * of another shape) is not a rejection and returns null, so it is never explained as one.
 */
export function admissionRejectionFromError(error: FetchJsonError): AdmissionRejection | null {
  if (error.kind !== "http" || error.status < 400 || error.status >= 500) return null;
  const body = error.body as { error?: { code?: unknown; message?: unknown } } | null | undefined;
  const code = body?.error?.code;
  if (typeof code !== "string" || !isAdmissionRejectionCode(code)) return null;
  const message = body?.error?.message;
  return { code, message: typeof message === "string" && message !== "" ? message : null };
}
