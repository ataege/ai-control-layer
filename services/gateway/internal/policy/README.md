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
   strictly against X-08, and the active revision through 3c's `catalog.Loader`, the single
   active-snapshot source).
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

## Denial feedback and the correction limit (GO-29, policy side)

- `BuildDenialFeedback(decision, scope)` gives the model the reason code, a fixed safe message
  per reason and, where one exists, a permitted alternative: the vendor report template after an
  export denial only when the passport permits that template, `create_report` and invoice
  sources; the passport's own invoice references after an out-of-scope read. It never carries a
  protected value, review content or the raw arguments, and the alternative grants nothing: the
  next proposal goes through the gate again.
- `CorrectionCounter.CorrectionsUsed` counts the run's denials from its durable decision events
  (`action.denied` and `report.export_denied`), so the count survives a worker restart and
  includes malformed proposals, which have no action row.
- `CheckCorrections(used, limit)` continues while used <= limit (a limit of 2 stops at the third
  denial) and otherwise returns `allowance_exhausted` as the stop reason.

f3's agent loop calls these after each denial and stops the run through
`repository.Tx.TransitionRun`; it keeps no counter of its own.

## Frozen review material (GO-43)

When the deterministic outcome (after the semantic check) is `approval_required`, the gate asks its
`ReviewFreezer` before recording the decision. Without a freezer, or when freezing fails, the
action is denied (`decision_unavailable`): nothing frozen means nothing to review.

`PostgresReviewFreezer` builds a `ReviewPayload` from trusted rows in one transaction and inserts
it into `runtime.review_payloads` (migration `1791080000000`, immutable rows, gateway SELECT and
INSERT only): the tool and canonical arguments, the passport and policy revision, the exact
recipient (Worker 2's `tools.ResolveRecipientForReview` on the stored passport's scope), and for
`queue_report` the stored report exactly as it would be queued (id, version, template and
projection versions, classification, content hash, content) with its sorted source manifest
(versions and consumed fields) and the manifest's digest. The review expires with the passport.

`payload_digest` is SHA-256 over the payload's canonical JSON, so a change to any material field is
detectable; the action's own digest stays untouched. `StaleSources` compares the frozen source
versions with the current ones by exact integer equality (the `record versions` rule), which GO-45
uses for `resource_version_changed`. The content and the address stay in restricted storage: the
`approval.requested` event carries only the report id, template and classification.

## Semantic action check (GO-77, policy side)

`SecurityActionEvaluator` adapts c1's `security.Inspector.EvaluateAction` to the gate's
`ActionEvaluator`. The gate calls it only after the deterministic checks allowed the action or sent
it to review (Figure 6), so a forbidden action never causes a security request. The inspector runs
the field limit, the signature rules and then the metered semantic check at the
`action_proposal` boundary:

- `no_objection`: the gate's outcome stays (allow, or approval required: never skipped);
- `block`: deny with the control's reason (for example `signature_match`), before any review
  material is frozen;
- `pause` or any failure: deny with the reason (`security_evaluator_unavailable` when none), which
  the worker treats as a pause.

`CatalogSecuritySettings` returns the settings of the active snapshot from 3c's `catalog.Loader`
when it is the action's evaluated revision; a different active revision, or a missing or
unenforceable snapshot (for example signature matching enabled with no feed bound), pauses. Every control record of the check is written
to `runtime.control_assessments` in the decision's transaction (one evaluation id, the action, the
admission and evaluated revisions, matched rule and feed revision, verdict and source for semantic
rows), never the inspected text.

## The approval decision (GO-44)

Routes (mounted by 3c's `internal/api` behind the service token and the verified operator
context): `ApprovalRoutePattern` (`POST /internal/actions/{actionId}/approval`, `ApprovalHandler`)
and `ReviewRoutePattern` (`GET /internal/actions/{actionId}/review`, `ReviewHandler`).

- The body is X-10 only (`contracts.ApprovalDecision`), `{"decision":"approve"|"reject"}`; any other field or a missing decision is
  `400` and nothing is decided.
- Reviewer authority comes from `app.memberships` for the verified user and organization (role
  `reviewer`, migration `1791110000000` grants the gateway read only); a signed claim alone never
  suffices, and a missing row or failed read denies.
- `Approvals.Decide` locks the action of the operator's organization (another organization's action
  is answered like a missing one), refuses a stopped or cancel-stamped run (`409 run_cancelled`,
  no grant, no continuation), requires `awaiting_approval`, the frozen expiry still ahead, the
  action digest recomputed from its stored arguments and the review payload digest recomputed from
  its stored content. Then one transaction writes the `runtime.approvals` row (bound to the action
  digest, the payload id, the reviewer and the frozen expiry), the action status `approved` or
  `rejected`, the `approval.decided` event and the continuation job (`repository.Tx.EnqueueJob`), or
  none of them. A second decision fails. f3's worker moves the run from `awaiting_approval` to
  `running` when it claims the job.
- `Approvals.FrozenReviewFor` returns the frozen payload (exact content and recipient) to a reviewer
  of the organization only.

## Executing an approved action (GO-45)

The executor runs an `approved` action as well as an `allowed` one. Before anything is written it
also checks that the active catalog revision is still the action's evaluated revision
(`source_policy_changed` otherwise), and for an approved action:

- the grant is approved, unconsumed and unexpired (`approval_expired` otherwise);
- the frozen sources' versions equal the current ones (`resource_version_changed`);
- the review material rebuilt from current rows with the same builder as the freeze (recipient
  resolved again, stored report and lineage) has the frozen digest, so a changed address, content,
  template or argument is `action_changed`.

In the effect's transaction the grant is consumed by the attempt before `RunEffect` (the database
guard allows one consumption of an approved, unexpired grant); nothing to consume means rollback
and `approval_expired`. `recordAttempt` claims the action first (allowed or approved to
`executing`), so a concurrent execution of the same action returns at once holding no lock, and
only then counts the run's attempts under `FOR NO KEY UPDATE` on the run row, the lock the event
writer also takes; the earlier order (run lock first) deadlocked with a running effect. A losing
concurrent execution is refused (`action_changed`).

## Retries and unknown outcomes (GO-53, executor half)

GO-53 belongs to Worker 2, who owns the effect transaction; the executor half lives here, built on
`tools.ClassifyRunError` and `tools.RetrySafe`:

- `RunEffect` returns an error: the transaction is rolled back, so no local effect committed. A
  retry-safe known no-effect failure releases the action (back to `allowed` or `approved`; a
  consumed grant was rolled back with it) and retries once under the same action id, through the
  normal checks and a new attempt that counts against the passport's limit. A precondition failure
  (the stored action, attempt or passport no longer matches) fails the action with an
  `action.failed` event and returns `stopped`, so the worker stops the run; no retry.
- The commit fails: the stored status is read back. `executed` or `failed` means the commit landed
  and the result is returned; otherwise the attempt stays open (the one-open-attempt index blocks
  any second attempt), the action becomes `unknown` with an `action.unknown` event
  (`outcome_unknown`), and the result is `paused`. Nothing re-runs it.
- An adapter refusal (`Outcome` failed) is `failed` with its reason, never retried.

## Labelled action replay (GO-36)

GO-05 chose a labelled Go runtime scenario: a stored prohibited proposal, taken from a hostile note
of the X-34 fixtures (`fixtures/hostile-notes.json`), is submitted for the next step of a named run
through the same `Gate.Evaluate` and `Executor.Execute` as a model proposal. There is no replay
branch; `Proposal.ReplaySource` (`labelled_replay:<fixture id>`) only labels the records:

- the stored action (`runtime.actions.replay_source`, migration `1791140000000`, X-09
  `StoredAction.replaySource`);
- every event the gate, the approval manager and the executor write for it
  (`maskedSummary.replaySource`).

`ReplayProposal(fixtureID, targets, ...)` builds the proposal a model would make if it obeyed the
note: `hostile_note_redirect_record_v1` reads `invoice_B01` (`resource_out_of_scope`),
`hostile_note_redirect_recipient_v1` queues the vendor report to the address in the note
(`destination_not_allowed`), `hostile_note_internal_disclosure_v1` queues the internal report to
the registered recipient (`report_export_restricted`). A replay makes no provider call and records
no model usage; a malformed label is denied (`invalid_arguments`).
