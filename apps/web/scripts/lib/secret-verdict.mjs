// How the end-to-end script judges the control layer's answer to a text that holds a secret.
// A verdict restricts and never grants, so the semantic check can still block a text after its
// redaction; that outcome depends on the model and the load and is only a note. A leaked secret,
// an allow, or a redaction without a marker is a failure. Every message carries decision/reasonCode.

/** Denials caused by the live semantic check or the load, not by a defect of the redaction. */
export const MODEL_DEPENDENT_DENIALS = [
  "semantic_injection_detected",
  "security_evaluator_unavailable",
  "security_allowance_exhausted",
];

/** @returns {{ status: "pass" | "note" | "fail", detail: string }} */
export function classifySecretVerdict(verdict, secretValue) {
  const outcome = `${verdict.decision}/${verdict.reasonCode}`;
  if (JSON.stringify(verdict).includes(secretValue)) {
    return { status: "fail", detail: `the secret value came back (${outcome})` };
  }
  if (verdict.decision === "allow") {
    return { status: "fail", detail: `the secret text was allowed (${outcome})` };
  }
  if (verdict.decision === "redact") {
    return verdict.content?.text?.includes("[REDACTED")
      ? { status: "pass", detail: "" }
      : {
          status: "fail",
          detail: `redact without a redaction marker in the returned text (${outcome})`,
        };
  }
  if (verdict.decision === "deny" && MODEL_DEPENDENT_DENIALS.includes(verdict.reasonCode)) {
    return {
      status: "note",
      detail: `denied (${outcome}) by the semantic check after redaction: model- or load-dependent, the text was withheld and no secret came back`,
    };
  }
  return { status: "fail", detail: `unexpected decision ${outcome}` };
}
