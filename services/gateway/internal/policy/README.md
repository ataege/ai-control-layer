# Action gate (`internal/policy`)

The enforcement package: it canonicalizes proposed actions, decides allow, deny or approval
required, and owns exact-action approvals. It is filled task by task (GO-12 first).

## Canonical arguments and the action digest (GO-12)

Implements GO-04's decision (`docs/product/README.md`, "GO-04: canonical arguments and action
digest").

- `DecodeArguments(tool, raw)` decodes a proposal's arguments strictly into the tool's typed form.
  Rejected before any canonical form exists: invalid UTF-8, a non-object, unknown keys, keys that
  differ only by case, duplicate keys, missing or null fields, wrong types (numbers included),
  trailing data, empty or over-long identifiers and control characters. Values are kept
  byte-for-byte: nothing is trimmed, case-folded or Unicode-normalized.
- `CanonicalArguments` encodes typed arguments as one compact JSON form with a fixed field order,
  so input field order, whitespace and equivalent escapes do not matter. Lists keep their order.
- `CanonicalAction.Digest()` is SHA-256 over the versioned canonical action:
  `canonicalization_version`, action and run ids, tool, canonical arguments, passport id, policy
  (catalog) revision id, recipient, affected resources with versions, exact outbound content and
  expiry (UTC, full precision). Absent optional values are `null`; an empty string is a value, not
  an absence. The digest and mutable execution status are not hashed.

The digest detects change only: "hashing a request does not authenticate its author or make its
contents authorized". Stored `canonical_arguments` (jsonb) reorder keys, so a recheck decodes the
stored arguments again and recomputes the digest; it never hashes stored bytes.

Argument shapes (proposed for X-09; renamed here if X-09 freezes different names):

| Tool            | Arguments                                                                           |
| --------------- | ----------------------------------------------------------------------------------- |
| `read_invoice`  | `invoice_id`                                                                        |
| `read_vendor`   | `vendor_id`                                                                         |
| `create_report` | `template` (one of the two fixed templates), `source_invoice_ids` (ordered, unique) |
| `queue_report`  | `report_id` (lowercase UUID), `recipient_reference` (trusted directory entry)       |

## The decision (GO-15)

`Gate.Evaluate(ctx, run, proposal)` returns exactly one of `allow`, `deny` or
`approval_required`, always with a decision; every failure is a deny. Order (Figure 6):

1. The proposal envelope (action id, step number, idempotency key, all from the worker) and the
   registered tool with strict arguments (GO-12). Malformed arguments cannot be canonicalized, so
   they get no `runtime.actions` row, only the denial event (`invalid_arguments`).
2. The passport scope of the verified run and the active catalog revision (`ScopeReader`;
   `PassportScopeReader` reads the stored passport through 3c's `repository.Passport`, decoded
   strictly against X-08, and the active revision through `config.ReadActiveAccountingCatalog`).
   A lookup failure, an invalid stored passport or a scope of another organization or run denies.
3. The action is stored (`proposed`) and committed before any further check.
4. Passport expiry, the passport's tools, then its resources: the invoice of `read_invoice`, the
   template and sources of `create_report`, the recipient reference of `queue_report`. Deeper
   relationships are GO-28, the export restriction GO-64.
5. The passport's approval rule turns an otherwise permitted action into `approval_required`.
6. The optional `ActionEvaluator` (GO-77) runs only after a deterministic allow or approval
   requirement. Its verdict can only deny; an error or unknown verdict denies; a semantic allow
   never skips review.

`PostgresRecorder` writes the records: `StoreAction` inserts the action (the same action stored
twice is accepted, any other conflict refused), and `RecordDecision` sets its status and appends
the decision's X-12 safe event through `repository.Tx.AppendEvent` in one transaction:
`action.allowed`, `approval.requested`, `action.denied`, or `report.export_denied` for a refused
export (with `lineageCheck` and the alternative template). Summaries hold references only. Without
a recorded decision the outcome is a deny (`decision_unavailable`).

Reason codes and tool names are the X-13 and X-09 constants of `internal/contracts`; an expired
passport is `run_expired`.

## The executor (GO-16)

`Executor.Execute(ctx, run, actionID)` runs one allowed action through Worker 2's
`tools.EffectRunner`, selected by the stored action's tool, never by the caller:

1. Fresh checks, before anything is written: the action belongs to this organization and run and
   is `allowed` (a denied, awaiting, executed or unknown action is refused); the run has no
   cancellation request and is not terminal; the stored passport is unexpired; the stored
   arguments still decode and recompute the stored digest (else `action_changed`).
2. The run's attempts are counted under a lock on the run row against the passport's tool attempt
   limit (`allowance_exhausted`), then the open attempt is inserted, the action set to
   `executing`, and both committed before dispatch.
3. One transaction: `RunEffect` (effect, attempt completion, audit event), then the action's
   final status `executed` or `failed`; commit.

Failure handling: an adapter or status error rolls back, so no effect was committed; the attempt is
closed as `aborted` and the result is `paused` (the action stays `executing` for reconciliation).
A failed commit is `paused` with `outcome_unknown`, and the attempt stays open, which blocks a
second attempt for the action until it is reconciled. Nothing is retried here.

The worker receives `tools.MinimizeForModel` output only; its untrusted text must still pass the
tool-result inspection (GO-76) before it becomes model context. GO-45 replaces step 2 with the
atomic tool reservation, attempt claim and approval consumption.

## Resource relationships and destinations (GO-28)

The gate checks every argument relationship of "Proposed tool argument boundaries" before an
adapter is reached; the adapters check again themselves.

| Tool            | Gate check                                                                                                                                              |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `read_invoice`  | The invoice is in the passport.                                                                                                                         |
| `read_vendor`   | The vendor belongs to the organization and is the vendor of a passport invoice (`RelationshipReader.VendorLinkedToInvoices`), not any vendor id.        |
| `create_report` | The template is one of the two registered templates and in the passport; every source invoice is in the passport.                                       |
| `queue_report`  | The recipient is a passport recipient reference that names this run, never content text; the report itself is checked by the export rule below (GO-64). |

A relationship that cannot be read denies (`decision_unavailable`), and a gate without a
relationship reader denies every action that needs one. `PostgresRelationships` reads the demo
records, always scoped by the verified organization. Whether a readable field may leave in
outbound content is the report's own restriction (GO-64), checked separately from read access.

## Export denial before review (GO-64)

For `queue_report`, after the resource checks and before the approval rule, the gate asks
`RelationshipReader.ReportExport`. `PostgresRelationships` implements it with Worker 2's
provenance package in one read-only transaction: `provenance.LoadReport` (report and lineage of
this organization and run), `provenance.CurrentInvoiceVersions`, then
`provenance.AuthorizeExport` for the registered vendor recipient. That decision uses the stored
lineage only, never the stored classification column, the title or a model label.

- No report of this organization and run: `resource_out_of_scope`.
- Content hash mismatch or missing lineage: `report_lineage_missing`.
- An Internal only lineage: `report_export_restricted`, a denial and never an approval request,
  with `vendor_reconciliation_v1` as the permitted alternative (recorded in the decision and its
  event summary; GO-29 offers it only when the passport permits it).
- A changed source version: `resource_version_changed`.

The semantic check is never reached for a restricted export. The `queue_report` adapter decides
provenance again at effect time, so an approval can never override the restriction.
