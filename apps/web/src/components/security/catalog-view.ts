// What the active-controls panel (WEB-29) shows, derived from one served CatalogStatus and from
// nothing else: which revision is in force, whether a requested change has taken effect, and each
// control's setting. Pure functions, tested without a browser.

import type { CatalogControl, CatalogStatus } from "@workspace/contracts";

import { countOf } from "@/lib/plural";

export interface ReloadRejection {
  code: string;
  message: string;
  stage: string;
  /** The revision that was rejected; 0 when the rejection names none (for example an invalid file). */
  revisionId: number;
}

/** Where a requested policy change stands, "so judges can see when a change is effective". */
export type ReloadState =
  /** No revision has ever been activated: nothing is enforceable yet. */
  | { kind: "none_active"; rejection: ReloadRejection | null }
  /** The active revision is the latest one requested, and nothing was rejected after it. */
  | { kind: "in_effect"; activeRevisionId: number }
  /** A newer revision was requested and has not been validated yet; the active one stays in force. */
  | { kind: "pending"; activeRevisionId: number; requestedRevisionId: number }
  /** The last activation or import was rejected; the active revision stays in force. */
  | { kind: "rejected"; activeRevisionId: number; rejection: ReloadRejection };

export function describeReload(status: CatalogStatus): ReloadState {
  const rejection = status.lastError;
  if (status.activeRevisionId === null) return { kind: "none_active", rejection };
  if (rejection !== null) {
    return { kind: "rejected", activeRevisionId: status.activeRevisionId, rejection };
  }
  if (
    status.requestedRevisionId !== null &&
    status.requestedRevisionId !== status.activeRevisionId
  ) {
    return {
      kind: "pending",
      activeRevisionId: status.activeRevisionId,
      requestedRevisionId: status.requestedRevisionId,
    };
  }
  return { kind: "in_effect", activeRevisionId: status.activeRevisionId };
}

/** A change is still on its way: worth asking again soon. */
export function isReloadPending(status: CatalogStatus): boolean {
  return describeReload(status).kind === "pending";
}

/** The first characters of a SHA-256 hex digest; the full value stays available as a title. */
export function shortDigest(digest: string | null): string {
  return digest === null ? "none" : `${digest.slice(0, 12)}…`;
}

export interface ControlRow {
  controlId: string;
  controlClass: "deterministic" | "semantic";
  enabled: boolean;
  /** What the control does when it fires. */
  response: string;
  threshold: string;
  boundaries: string[];
}

function responseOf(control: CatalogControl): string {
  if (!control.enabled) return "not applied";
  if (control.mode === "block") return "block";
  if (control.mode === "redact") return "redact";
  // No mode on an enabled control: the signature feed's own rules each carry their response.
  return control.controlId === "signature_match" ? "set by each feed rule" : "not stated";
}

export function controlRows(status: CatalogStatus): ControlRow[] {
  return status.controls.map((control) => ({
    controlId: control.controlId,
    controlClass: control.controlClass,
    enabled: control.enabled,
    response: responseOf(control),
    threshold: control.threshold === null ? "none" : String(control.threshold),
    boundaries: control.enabled ? control.boundaries : [],
  }));
}

/**
 * The signature feed the active catalog is bound to, in words: its revision, the feed revision id and
 * how many rules it has ("1 rule", "2 rules"; an unserved count is said to be unknown, not "none").
 */
export function describeSignatureFeed(
  status: Pick<CatalogStatus, "feedRevision" | "feedRevisionId" | "feedRuleCount">,
): string {
  if (status.feedRevision === null) return "none bound";
  const revisionId = status.feedRevisionId === null ? "none" : String(status.feedRevisionId);
  const rules =
    status.feedRuleCount === null ? "rule count unknown" : countOf(status.feedRuleCount, "rule");
  return `${status.feedRevision} (revision ${revisionId}, ${rules})`;
}
