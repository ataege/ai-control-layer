import type {
  LedgerTokens,
  ModelLedger,
  PurposeUsage,
  RunState,
  RunUsage,
} from "@workspace/contracts";
import { estimatedCostLabel, type LabelText } from "@/lib/labels";

// Pure view model of a run's model usage (WEB-16, WEB-19, WEB-17). Reported usage (what the provider
// measured and the gateway settled), reserved allowance (held before and during a request) and cost
// are different figures and are never added together. Unknown usage is uncertain, never zero.

const NUMBER = new Intl.NumberFormat("en-US");

export function formatCount(value: number): string {
  return NUMBER.format(value);
}

const PURPOSE_LABEL: Record<PurposeUsage["purpose"], string> = {
  agent: "Agent steps",
  security: "Security checks",
};

export interface PurposeRow {
  purpose: string;
  label: string;
  dispatched: number;
  completed: number;
  failed: number;
  inFlight: number;
  /** Requests whose usage could not be settled; their reservation stays held. */
  unknownCalls: number;
  /** Tokens the provider reported for settled requests. */
  reportedTokens: number;
  /** Tokens reserved for requests in flight or with unknown usage; not usage. */
  reservedTokens: number;
  uncertain: boolean;
  /** Every dispatched request has an outcome, a held reservation or is still in flight. */
  accounted: boolean;
}

export interface AllowanceRow {
  name: string;
  /** Null: no limit of its own beyond the shared total. */
  limit: number | null;
  reported: number;
  reserved: number;
  /** Limit minus reported minus reserved, never below zero; null without a limit. */
  available: number | null;
}

export interface CallRow {
  name: string;
  used: number;
  limit: number;
  reached: boolean;
}

export interface LedgerView {
  paused: boolean;
  allowances: AllowanceRow[];
  calls: CallRow[];
  requestTimeoutSeconds: number;
  maxConcurrentCalls: number;
  callsInFlight: number;
}

export interface UsageView {
  runId: string;
  purposes: PurposeRow[];
  /** Null when the run has no ledger yet: nothing was reserved or reported. */
  ledger: LedgerView | null;
  reportedTokens: number;
  reservedTokens: number;
  unknownCalls: number;
  uncertain: boolean;
  /** The shared label for an unresolved reservation; null when nothing is uncertain. */
  uncertainLabel: LabelText | null;
  /** Cost is never shown unless a documented pricing rule exists; the contract carries none. */
  costNote: string;
  toolAttempts: RunUsage["toolAttempts"];
}

function purposeRow(usage: PurposeUsage): PurposeRow {
  const unknownCalls = Math.max(usage.usageUnknown, usage.usageUnknownReservations);
  return {
    purpose: usage.purpose,
    label: PURPOSE_LABEL[usage.purpose] ?? usage.purpose,
    dispatched: usage.dispatched,
    completed: usage.completed,
    failed: usage.failed,
    inFlight: usage.inFlight,
    unknownCalls,
    reportedTokens: usage.settledTokens,
    reservedTokens: usage.heldTokens,
    uncertain: unknownCalls > 0,
    accounted:
      usage.dispatched === usage.completed + usage.failed + usage.usageUnknown + usage.inFlight,
  };
}

function allowance(name: string, tokens: LedgerTokens): AllowanceRow {
  return {
    name,
    limit: tokens.limit,
    reported: tokens.used,
    reserved: tokens.reserved,
    available:
      tokens.limit === null ? null : Math.max(tokens.limit - tokens.used - tokens.reserved, 0),
  };
}

function ledgerView(ledger: ModelLedger): LedgerView {
  const call = (name: string, used: number, limit: number): CallRow => ({
    name,
    used,
    limit,
    reached: used >= limit,
  });
  return {
    paused: ledger.paused,
    allowances: [
      allowance("Shared tokens (both purposes)", ledger.tokens),
      allowance("Agent tokens", ledger.agentTokens),
      allowance("Security tokens", ledger.securityTokens),
    ],
    calls: [
      call(
        "Model requests (both purposes)",
        ledger.calls.agent + ledger.calls.security,
        ledger.calls.limit,
      ),
      call("Agent requests", ledger.calls.agent, ledger.calls.agentLimit),
      call("Security requests", ledger.calls.security, ledger.calls.securityLimit),
    ],
    requestTimeoutSeconds: ledger.requestTimeoutMilliseconds / 1000,
    maxConcurrentCalls: ledger.maxConcurrentCalls,
    callsInFlight: ledger.callsInFlight,
  };
}

/**
 * The usage of one run (X-29). Limits come from the run's own ledger, which the passport set, never
 * from a constant here. A request with unknown usage makes the figures uncertain; it is never shown as
 * zero, and no cost is shown because the contract carries none (a local model, no pricing rule).
 */
export function describeUsage(usage: RunUsage): UsageView {
  const purposes = usage.modelCalls.map(purposeRow);
  const unknownCalls = purposes.reduce((total, row) => total + row.unknownCalls, 0);
  const uncertain = unknownCalls > 0;
  return {
    runId: usage.runId,
    purposes,
    ledger: usage.ledger === null ? null : ledgerView(usage.ledger),
    reportedTokens: purposes.reduce((total, row) => total + row.reportedTokens, 0),
    reservedTokens: purposes.reduce((total, row) => total + row.reservedTokens, 0),
    unknownCalls,
    uncertain,
    uncertainLabel: uncertain ? estimatedCostLabel({ unresolved: true }) : null,
    costNote:
      "Estimated cost is not shown: this run uses a local model and no pricing rule is configured, so no amount is stated.",
    toolAttempts: usage.toolAttempts,
  };
}

export type LimitStopKind = "model_allowance" | "security_allowance" | "time";

const LIMIT_REASONS: Record<string, LimitStopKind> = {
  allowance_exhausted: "model_allowance",
  security_allowance_exhausted: "security_allowance",
  run_expired: "time",
};

/**
 * The limit-triggered stop of a run (WEB-17), read from the persisted state alone: paused or stopped
 * with a terminal reason that names a limit. Anything else is not a limit stop.
 */
export function limitStopKind(run: RunState): LimitStopKind | null {
  const status: string = run.status;
  if ((status !== "paused" && status !== "stopped") || run.terminalReason === null) {
    return null;
  }
  return LIMIT_REASONS[run.terminalReason] ?? null;
}
