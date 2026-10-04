# Control policy configuration

`policy.yaml` in this directory is the documented, operator-editable sample of the Task Passport
control policy (roadmap task SH-42, interface X-78). Report version 1.2, "Central policy
configuration and safe reload": "The required sample configuration would be a documented
policy.yaml imported through NestJS into one central immutable control-catalog version."

This schema is the "policy activation and catalog revision" contract, owned by the web + API
implementer (`docs/product/README.md`, "Contracts to freeze first"). It is on `main` and enforced:
the importer (`apps/api/src/policies/policy-file.ts`) and Go (`services/gateway/internal/security`
and `internal/catalog`) both parse the whole file against it.

## How the file is used

| Step     | What happens                                                                                                                                                                                                                                                                                                                                         | Owner         |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| Edit     | An authorized policy owner or judge edits `policy.yaml`.                                                                                                                                                                                                                                                                                             | operator      |
| Import   | An explicit command (API-32) or the authenticated reload (API-33) parses the whole file against the schema below, records its SHA-256 digest and stores it as a new immutable catalog revision. Nothing imports it at application startup.                                                                                                           | NestJS        |
| Validate | "Go fetches and validates the candidate, acknowledges readiness" (GO-73): Go validates the whole candidate against this shared schema, and refuses one it cannot enforce. It does not check that a model is installed (see "Known limitations"). The protocol is item 15 of "Decisions recorded by the lead's delegate" in `docs/product/README.md`. | Go            |
| Evaluate | Go reads the active revision before each new evaluation and dispatch and records the admission and evaluated revision in every decision (GO-72).                                                                                                                                                                                                     | Go            |
| Reject   | An invalid file never partially activates. The last accepted revision stays active and the rejection reason is visible (`policy_reload_rejected`). With no valid initial revision the gateway is not ready and dispatches nothing.                                                                                                                   | NestJS and Go |

The file is an import input, "not a second configuration authority": after an import, the stored
revision is what counts, and editing the file changes nothing until the next accepted import.

## Importing the file

From the repository root, with the database migrated (`pnpm db:migration:run`):

```sh
pnpm policy:import                 # imports config/policy.yaml
pnpm policy:import path/to/copy.yaml
```

The command (API-32) validates the whole file and the signature feed it names (`signatures.path`,
next to the policy file; the feed half of API-34), and then, in one transaction:

- **Valid file and feed:** stores a new immutable revision (`app.control_catalog_revisions`) and the
  feed's exact bytes with their SHA-256 (`app.signature_feed_revisions`; the same issuer, revision
  and bytes reuse the stored row), and makes the revision the requested revision. It prints
  "requested revision N; the gateway validates and activates it" and exits 0. The import never
  activates: the gateway validates the requested revision and switches the active revision and feed
  together (GO-73, `catalog activation protocol`). Without a running gateway, `pnpm catalog:activate`
  runs that activation once (the test database and `pnpm reset:demo` do). With a running gateway its
  watcher activates a requested revision within a few seconds; until a revision is active, a fresh
  database has no active catalog and the gateway stays not ready.
- **Unchanged file and feed:** a no-op. When the policy digest and the feed (issuer, revision,
  digest) equal the current revision (the requested one, else the active one), the command prints
  "unchanged: revision N is already current", writes nothing and exits 0. A new revision with the same
  digest would move the pointer, and pending work (an approval waiting for review) is bound to the old
  revision, so the executor would refuse it as `source_policy_changed`: a judge's repeated import of
  an unchanged file must not void an approval. Only a sound current revision counts (validated by the
  gateway, or requested and still waiting for it); a request the gateway rejected, or an active
  revision an old import bootstrapped without validation, gets a fresh revision, which is how it is
  retried or healed.
- **A pending request:** a valid, changed file is refused while the requested revision has not been
  checked by the gateway yet (it differs from the active and the validated revision and has no
  recorded gateway rejection). Nothing is written; the command prints "revision N is still being
  validated; wait for activation and retry" and exits 1 (without a running gateway, run
  `pnpm catalog:activate`). Otherwise two imports inside one activation tick could replace a good
  edit before it was ever validated. A file's own problems are reported first.

  Two deliberate exceptions to the plain rules "an identical file is a no-op" and "refuse while
  requested is not validated", both so the recovery path stays open:
  - An identical file is not a no-op when the current revision is not sound (the gateway rejected
    it, or an old import bootstrapped it without validation). Treating it as unchanged would make
    `catalog:activate`'s advice to "import the policy again" loop forever, and would make a rejected
    request impossible to retry with the same file after its cause (for example a missing feed) is
    fixed.
  - The refusal does not apply when the requested revision is the active one (the state an old
    import's first-revision bootstrap leaves: requested = active, validated empty). The plain rule
    would be true there too and would block the one import that heals that state.

- **Invalid file or feed:** stores nothing. The issues are always printed (and returned) and the
  command exits 1; it also records the reason (`policy_reload_rejected`), the file digest and up to 20
  issues on the pointer (`app.control_catalog_pointer`), except when the gateway's own rejection of
  the still-requested revision is pending there: that record stays, because the gateway only
  recognizes its own and would otherwise validate and reject the same revision again. The active
  revision stays. A feed is refused when Go would refuse it (`ParseFeed` grammar), when its
  revision differs from `signatures.revision`, when a `disabled_rules` ID is not in it, when the same
  feed revision is already stored with different bytes (the message names the usual cause, a feed loaded by hand: recreate the database, or bump the revision if the rules really changed), or when it is missing while
  `signature_match` is enabled.

Before the schema is checked, the file must be at most 64 KiB of valid UTF-8 without control
characters (tab, line feed and carriage return are allowed) and hold one plain YAML document: no
duplicate keys, no aliases and no custom tags. A leading byte order mark is kept in the stored text, so
the stored text always matches the stored digest. A relative path is relative to the repository root.
Rejection messages are fixed texts per kind of problem; they never repeat a key or value from the
file. The command never runs at application
startup.

## Values

Every number in the sample is an illustrative value from the project report, labelled there as
"illustrative team settings, not sponsor requirements or measured performance". The team fixes the
real values at the M0 freeze (X-06); the sample values are that policy fixture. Limits "depend on the
selected model and hardware and require measurement". The model `qwen3.5:4b` through Ollama is frozen
for the demonstration (model decision, GO-03), and the probe that chose it is in `docs/setup.md`, section 7.

## Relation to the report's sample

The schema keeps the shape of the report's appendix sample ("Documented policy file and reload
contract") and adds keys for settings the report describes elsewhere. The appendix text itself is not
a valid file under this schema, because these keys are required:

| Added key                              | Report basis                                                                                                                  |
| -------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `budgets.tool_attempts`, `corrections` | The illustrative passport fields `tool_attempt_limit` (12) and `correction_limit` (2)                                         |
| `controls.*.boundaries`                | "Proposed hybrid evaluation boundaries"; "Inspect configured untrusted input/output"                                          |
| `signature_match`, `disabled_rules`    | "Disabling a sample signature rule, adding another rule or changing a semantic threshold must produce an observable revision" |
| `reports.enabled_templates`            | "Fixed templates/projections" in the reports and audit group; a team addition that can only narrow                            |

## Schema

The schema is closed. A missing required key, a key that is not listed here, a value of the wrong
type or a value outside its allowed set rejects the whole file. Strings are case-sensitive. No key
has a default, except the three backward-compatible GO-06 accounting additions below.
All other keys in the tables are required.

### Top level

| Key              | Type            | Rule                                                               |
| ---------------- | --------------- | ------------------------------------------------------------------ |
| `schema_version` | integer         | Must be `1`, written as `1`. A future schema change increments it. |
| `allowed_models` | list of strings | Models and budgets group, see below.                               |
| `budgets`        | mapping         | Models and budgets group, see below.                               |
| `controls`       | mapping         | Controls group, see below.                                         |
| `signatures`     | mapping         | Attack feed group, see below.                                      |
| `reports`        | mapping         | Reports and audit group, see below.                                |

### Models and budgets

Report: "Allowed local model references; shared and purpose-specific calls/tokens/time; concurrency"
Invariant: "Every agent and security dispatch is metered"

`allowed_models` is a non-empty list without duplicates. Each entry is the model tag as the local
provider names it (for example an Ollama tag), matching `^[a-z0-9][a-z0-9_.:-]{0,63}$`. Every listed
model may serve both metered purposes, agent and security; "Purpose is assigned by trusted runtime
code" in Go, never by the file or the model.

NestJS validates the syntax of each entry. Whether a model is installed is known only where the
model runs, and nothing checks it at import or activation: a tag the provider does not have activates
and fails closed at the first model call (see "Known limitations"). Every model call is checked
against the active list and denied before dispatch (`model_not_allowed`) when its model is not in
it, even on a previously admitted run.

`budgets`, all positive integers written in plain decimal digits (zero, negative or fractional values, and spellings such as `24.0`, `0x18` or `0o30`, reject the file):

| Key                       | Sample | Meaning                                                                                                                                                                                            |
| ------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `calls_total`             | 24     | Model calls per run across both purposes.                                                                                                                                                          |
| `calls_agent`             | 12     | Ceiling for agent-purpose calls. Must not exceed `calls_total`.                                                                                                                                    |
| `calls_security`          | 12     | Ceiling for security-purpose (semantic check) calls. Must not exceed `calls_total`.                                                                                                                |
| `tokens_total`            | 40000  | Input and output tokens per run across both purposes. Raised from the report's illustrative 20000 on 2026-10-03, because the live story re-sends its growing context and ran out at its 10th call. |
| `agent_output_tokens`     | 512    | Maximum agent output, sent as Ollama `options.num_predict`. Optional in older v1 revisions; Go defaults to 512.                                                                                    |
| `security_output_tokens`  | 256    | Maximum semantic-guard output. Optional in older v1 revisions; Go defaults to 256.                                                                                                                 |
| `input_template_tokens`   | 1024   | Conservative template allowance added to the JSON UTF-8 input byte count. Optional in older v1 revisions; Go defaults to 1024.                                                                     |
| `request_timeout_seconds` | 20     | Deadline for one model request. Must be shorter than `run_expiry_minutes` (in seconds).                                                                                                            |
| `local_max_concurrency`   | 2      | Concurrent local model requests.                                                                                                                                                                   |
| `run_expiry_minutes`      | 15     | Run lifetime from admission.                                                                                                                                                                       |
| `tool_attempts`           | 12     | Governed tool attempts per run, safe retries included.                                                                                                                                             |
| `corrections`             | 2      | Bounded feedback rounds after a denied proposal.                                                                                                                                                   |

Counts must also fit a JavaScript safe integer; request timeout is at most 86,400 seconds.
The user adopted the accounting settings above on 3 October 2026 for GO-06. Every accounted
request uses `think: false` and `stream: false`. Agent and security share `tokens_total`.
Input reservation includes system messages, conversation history, tool results, tool calls,
tool definitions and response schemas. Valid completed usage settles both reported counts;
missing counts and timeout retain the whole reservation as `usage_unknown`. An overrun records
the full measured usage and pauses further dispatch. There is no automatic timeout retry.
The byte estimate is conservative on the measured fixtures, not a proven tokenizer bound.

The purpose sub-limits are ceilings, "not reserved entitlements": "Both purposes count toward the same
totals." Each sub-limit must be less than or equal to `calls_total`; their sum may exceed it. A call is
reserved against both the shared ceiling and its purpose ceiling before dispatch, so security calls
"cannot escape the task ceiling".

Budget changes apply as current restrictions. Lowering a budget constrains future dispatches of
running tasks; the one exception is `run_expiry_minutes`, which is fixed into the passport's
expiry at admission and so applies to the next run (see "Where each setting takes effect"). Raising one never exceeds the ceiling stored in an admitted passport; "an expanded
resource, destination, model or budget grant requires renewed admission".

### Controls

Report: "Enabled guard IDs; block or redact response; semantic risk thresholds" Invariant: "Optional
settings never grant source or task authority"

`controls` holds exactly the three registered guard IDs below, all required; any other ID rejects the
file ("Only registered adapters and supported policy controls would be executable").

| Guard ID             | Kind                                       | Keys                                         | Supported `boundaries`                          |
| -------------------- | ------------------------------------------ | -------------------------------------------- | ----------------------------------------------- |
| `secret_pattern`     | deterministic secret and PII rules (GO-74) | `enabled`, `mode`, `boundaries`              | `model_input`, `tool_result`                    |
| `semantic_injection` | AI-based semantic evaluator (GO-75)        | `enabled`, `mode`, `threshold`, `boundaries` | `model_input`, `tool_result`, `action_proposal` |
| `signature_match`    | deterministic signature feed matching      | `enabled`, `boundaries`                      | `model_input`, `tool_result`, `action_proposal` |

| Key          | Type            | Rule                                                                                                                                         |
| ------------ | --------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `enabled`    | boolean         | `false` explicitly disables the optional guard; the change is recorded in the new revision. All keys stay required when a guard is disabled. |
| `mode`       | string          | The response when the guard fires: `block` or `redact`.                                                                                      |
| `threshold`  | number          | The semantic risk cutoff, from 0 to 1 inclusive (the verdict score range); a score at or above it fires the guard. See the note below.       |
| `boundaries` | list of strings | Where the guard runs. Non-empty, no duplicates, each one of the values the guard supports.                                                   |

The boundaries are the report's "Proposed hybrid evaluation boundaries": before model dispatch
(`model_input`), tool result to agent context (`tool_result`) and the proposed action
(`action_proposal`). Live agent runs inspect tool results and action proposals; the `model_input`
boundary runs on the judge's test entry (see "Known limitations").

Notes on each guard:

- `secret_pattern`: covers the report's "Configured secret/PII patterns". Where the patterns are
  defined, which fields they apply to and how match spans are validated is recorded in the
  `redaction rules` decision (items 8 and 25 of "Decisions recorded by the lead's delegate" in
  `docs/product/README.md`); this schema carries no pattern or regular expression.
- `semantic_injection`: the 0.75 cutoff "is an illustrative score cutoff, not a calibrated
  probability". Go validates the verdict's schema and "applies policy thresholds itself". The score
  range is 0 to 1 and a score at or above the threshold fires (item 7 of that list; the importer and
  Go both enforce the range). Semantic suspicion
  in `redact` mode "may cause the server to mask the whole configured field rather than accepting
  free-form rewritten text". At `action_proposal`, "Allow, deny and require approval remain the
  principal action outcomes; redaction applies to supported content fields", so what `redact` means
  for a proposal is recorded in the `redaction rules` decision. The security request is metered against
  `calls_security` and the shared ceilings, and does not trigger another semantic check.
- `signature_match`: matches the rules of the feed named in `signatures`. Each feed rule carries its
  own response (report: "versioned rule data carrying issuer, revision, scope, pattern type, response
  and integrity metadata"), so this guard has no `mode`.

A deliberately disabled guard (`enabled: false`) is an authorized configuration change. An
operational failure of an enabled guard is not: "timeout, unavailable model, malformed response or
exhausted safety allowance pauses or denies the interaction; the gateway never silently forwards
unchecked text or executes the action." That rule is fixed, not a setting.

### Attack feed

Report: "attack-signatures.json; trusted issuer/key and revision/digest" Invariant: "Only validated
data rules; no downloaded executable code"

| Key              | Type            | Rule                                                                                                                                                                                                |
| ---------------- | --------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `path`           | string          | A bare file name in this directory, matching `^(?!.*\.\.)[A-Za-z0-9_-][A-Za-z0-9_.-]*\.json$`: no URL, no directory separator, no `..`. The feed is a local file; network fetching is out of scope. |
| `revision`       | string          | The feed revision this policy binds to, matching `^[a-z0-9][a-z0-9_.-]{0,63}$`.                                                                                                                     |
| `disabled_rules` | list of strings | Rule IDs from the bound feed that are switched off in this revision; may be empty, no duplicates. Each ID matches `^[a-z0-9][a-z0-9_]{0,63}$`.                                                      |

The issuer and the feed digest are properties of the feed file and its import, not of this policy.
The import reads the feed named by `path` (`attack-signatures.json`, SH-46) and validates it the way
Go does: the closed grammar (`normalized_substring` data rules, response `block`), its size, the feed
revision equal to `revision`, and each `disabled_rules` ID present in the feed. It stores the exact
bytes with their SHA-256; Go accepts only those bytes. There is no signing key: trust is the
authenticated import plus the digest pin. Grammar and rule table: `services/gateway/README.md`.

The report says "The same activation command validates the referenced versioned attack-signatures.json
feed. A failed validation leaves the accepted active version intact". As decided (item 15 of that list): a candidate whose enabled `signature_match` references a feed
revision that has not been accepted is invalid and does not activate, so the last-known-good revision
stays active; it never runs as an empty rule set that matches nothing.

Disabling a rule is an optional detection change: it produces a new revision and changes the
recorded decisions, and leaves scope and report restrictions untouched.

### Reports and audit

Report: "Fixed templates/projections; authorized destinations; safe export fields" Invariant:
"Internal only cannot be cleared by a classifier or approval"

| Key                         | Type            | Rule                                                                                                |
| --------------------------- | --------------- | --------------------------------------------------------------------------------------------------- |
| `reports.enabled_templates` | list of strings | May be empty, no duplicates, each one of `internal_investigation_v1` or `vendor_reconciliation_v1`. |

`enabled_templates` can only narrow. It is a current restriction like a lowered budget: removing a
template forbids new reports from it, and a queue attempt for an existing report from that template
is denied at the gate with `template_not_allowed`, before any review is requested. An action that was
already approved is refused at execution as `source_policy_changed`, because every catalog change
after an action was evaluated voids it. A template listed here still needs the passport to allow it.

The other settings the report names for this group are not in the schema yet, and the schema holds no
values for them:

- Projections: the report templates and the projection rule are Go constants
  (`services/gateway/internal/provenance`); the vendor projection is `vendor_invoice_fields_v1` (item 5
  of the delegate's list, `vendor projection fields`).
- Authorized destinations: the vendor's registered reporting address on `demo.vendors` and the
  `internal_note_classification` column on `demo.invoices` (item 6, `source classification storage`).
- Safe export fields: set by the security summary and audit export record contract (web + API
  implementer). JSON and CSV export are fixed capabilities, not settings.

Go derives every classification from trusted records; no setting here classifies or declassifies a
report.

## Where each setting takes effect

Audited against the Go code on 2026-10-04 (branch `go/w3-policy-audit`). Go reads the active revision
again for every decision: the pointer is read on each snapshot read and only the parse is cached per
immutable revision (`catalog/catalog.go:90`). "Next call" means the next model call or gateway
decision of a running run; "next run" means the next admission. Every value is validated twice, at
import (`apps/api/src/policies/policy-file.ts`) and by Go at activation (`catalog/activation.go`
through `buildSnapshot`, `catalog.go:127`). A value Go cannot parse is `ErrUnavailable`: the revision
does not activate, and a stored revision that cannot be parsed dispatches nothing. A raised value
never exceeds the stored passport (`EffectiveFor`, `catalog.go:243`).

| Setting                                                                          | Go reads it                                                                                                  | Takes effect                                                                                     | Invalid value                                                                                                                                                                                                                           | Test                                                                                                                                                                                                                                 |
| -------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `schema_version`                                                                 | `ParseLimits`, `catalog.go:171`                                                                              | Activation: a revision not at `1` never activates                                                | Rejected at import and by Go                                                                                                                                                                                                            | `TestParseLimitsFailsClosed` ("wrong schema version")                                                                                                                                                                                |
| `allowed_models`                                                                 | `ParseLimits`; per call `agent/chain.go:212` and `agent/step.go:100`; passport intersection `EffectiveFor`   | Next call, also for an admitted run                                                              | Empty list rejected. Syntax is checked at import only; installation is not checked (F3)                                                                                                                                                 | `TestTheActiveCatalogNarrowsEveryStep` ("model removed by the catalog"), `TestParseLimitsFailsClosed` ("no models")                                                                                                                  |
| `budgets.calls_total`, `calls_agent`, `calls_security`                           | `ParseLimits`; ledger ceiling `agent/chain.go:224`; agent steps `agent/loop.go:283`                          | Next call, lowered or raised, never above the passport                                           | At least 1, sub-limits at most the total, at most 2^53-1: rejected at import and by Go                                                                                                                                                  | `TestPostgresReserveWithinNarrowsButNeverWidens`, `TestModelGatewayAppliesALoweredCatalogLimitToARunningPassport`, `TestParseLimitsFailsClosed` (sub-limits, range)                                                                  |
| `budgets.tokens_total`                                                           | `ParseLimits`; ledger ceiling `agent/chain.go:225`                                                           | Next call                                                                                        | At least 1: rejected at import and by Go                                                                                                                                                                                                | `TestPostgresReserveWithinNarrowsButNeverWidens` (token ceiling), `TestParseLimitsFailsClosed` ("zero tokens total")                                                                                                                 |
| `budgets.request_timeout_seconds`                                                | `ParseLimits`; `agent/chain.go:225` and `:233`                                                               | Next call                                                                                        | At least 1 and shorter than `run_expiry_minutes` in seconds: rejected at import and by Go                                                                                                                                               | `TestPostgresReserveWithinNarrowsButNeverWidens` (timeout ceiling), `TestParseLimitsFailsClosed` ("timeout beyond expiry")                                                                                                           |
| `budgets.local_max_concurrency`                                                  | `ParseLimits`; slot count `agent/chain.go:240`                                                               | Next call                                                                                        | At least 1: rejected at import and by Go                                                                                                                                                                                                | `TestModelGatewaySlotWaitKeepsTheFullRequestTime` (slots from the snapshot), `TestParseLimitsFailsClosed` ("zero local concurrency")                                                                                                 |
| `budgets.run_expiry_minutes`                                                     | `ParseLimits`; `admission/admission.go:277` and `:313`, `admission/options.go:54`                            | Next run: the passport keeps its own `expires_at` from admission                                 | At least 1: rejected at import and by Go. Go rejects a lower value twice, by the positive-integer check and by the rule that the request timeout is shorter than the expiry; no input isolates the first (see the note below the table) | `admission_test.go:133` (expiry from admission), `TestParseLimitsFailsClosed` ("zero run expiry", rejected by both rules)                                                                                                            |
| `budgets.tool_attempts`                                                          | `ParseLimits`; `policy/scope.go` `LoadScope` through `EffectiveFor`, used at `policy/executor.go:162`        | Next call, lowered or raised, never above the passport (F1, fixed)                               | At least 1: rejected at import and by Go                                                                                                                                                                                                | `TestToolAttemptLimitFollowsTheActiveCatalogButNeverWidensThePassport`, `TestLoweredToolAttemptsRefuseTheNextExecutionWithAllowanceExhausted` (end to end through the executor), `TestParseLimitsFailsClosed` ("zero tool attempts") |
| `budgets.corrections`                                                            | `ParseLimits`; `agent/loop.go:461` through `EffectiveFor`                                                    | Next call, lowered or raised, never above the passport                                           | Import requires at least 1; Go also accepts 0 and rejects negative or fractional values                                                                                                                                                 | `TestEffectiveLimitsNarrowButNeverWidenThePassport`, `TestParseLimitsFailsClosed`. Not tested: lowering it through the catalog during a run                                                                                          |
| `budgets.agent_output_tokens`, `security_output_tokens`, `input_template_tokens` | Accounting reader per call, `agent/chain.go:203` to `:228`, with a same-revision check                       | Next call                                                                                        | Absent takes the default; present must be a positive integer (null, 0, fractions rejected)                                                                                                                                              | `TestParseLimitsReadsTheAccountingFieldsWithTheirDefaults`, `TestModelGatewayRefusesACallAcrossTwoCatalogRevisions`                                                                                                                  |
| `controls.*.enabled`                                                             | `security.SettingsFromCatalog`, `security/catalog.go:41`; applied by `appliesAt`, `security/security.go:131` | Next step: tool results, action proposals and the judge entry                                    | Missing or non-boolean: rejected at import and by Go (`ErrSettings`)                                                                                                                                                                    | `TestSettingsFromCatalogRejectsUnenforceableCatalogs` ("missing enabled"), the `enabled` cases in `content_test.go`, `signatures_test.go`, `semantic_test.go`                                                                        |
| `controls.secret_pattern.mode`, `controls.semantic_injection.mode`               | `guardFromCatalog`, `security/catalog.go:102`; `security/content.go:205`, `security/semantic.go:312`         | Next step                                                                                        | Only `block` or `redact`; `allow` or a missing mode is rejected                                                                                                                                                                         | `TestSettingsFromCatalogRejectsUnenforceableCatalogs` ("mode allow"), `TestEvaluateAppliesThresholdInGo` (redact masks the field)                                                                                                    |
| `controls.semantic_injection.threshold`                                          | `security/catalog.go:102`; compared at `security/semantic.go:309`: a score at or above it fires              | Next step                                                                                        | Outside 0 to 1, missing or not a number: rejected at import and by Go                                                                                                                                                                   | `TestEvaluateAppliesThresholdInGo` (at the threshold blocks), `TestSettingsFromCatalogRejectsUnenforceableCatalogs` ("threshold out of range")                                                                                       |
| `controls.*.boundaries`                                                          | `security/catalog.go:102`; `appliesAt` at `inspect.go:224`, `action.go:86` and `evaluation.go:209`           | Next step. `model_input` applies on the judge entry only (F4)                                    | Empty, duplicate or unsupported values rejected at import and by Go                                                                                                                                                                     | `TestSettingsFromCatalogRejectsUnenforceableCatalogs` ("empty boundaries", "secret at action proposal"), `Boundaries` cases in `inspect_test.go`, `action_test.go`                                                                   |
| `signatures.path`                                                                | `security/catalog.go:41`: presence only; the importer reads the file                                         | Import only: Go uses the feed bytes stored at import                                             | A URL, a directory part or `..` is rejected at import; a missing key is rejected by Go                                                                                                                                                  | `policy-file.spec.ts` (path cases), `TestSettingsFromCatalogRejectsUnenforceableCatalogs` ("missing signatures")                                                                                                                     |
| `signatures.revision`                                                            | `security/catalog.go:41`: must equal the stored feed's revision                                              | Activation: a mismatch never activates                                                           | Rejected at import and by Go (`ErrFeed`)                                                                                                                                                                                                | `TestSettingsFromCatalogRejectsUnenforceableCatalogs` ("feed revision differs")                                                                                                                                                      |
| `signatures.disabled_rules`                                                      | `security/catalog.go:41`; skipped per rule at `security/signatures.go:210`                                   | Next step                                                                                        | Unknown or duplicate rule IDs rejected at import and by Go                                                                                                                                                                              | `TestSettingsFromCatalogRejectsUnenforceableCatalogs` ("disabled rule not in feed", "duplicate disabled rule"), `signatures_test.go`                                                                                                 |
| `reports.enabled_templates`                                                      | `ParseLimits`; `EffectiveFor`; `policy/scope.go` `LoadScope`; queue check in `policy/gate.go`                | Next call: new `create_report` and queue of an existing report are denied `template_not_allowed` | Unknown template rejected at import and by Go; an empty list is allowed                                                                                                                                                                 | `TestTemplateRevokedThroughTheCatalogNarrowsTheScope`, `TestQueueOfAReportFromADisabledTemplateIsDeniedBeforeReview`, `TestParseLimitsFailsClosed` ("unknown template")                                                              |

A note on `budgets.run_expiry_minutes`: any value below 1 is also below every valid request timeout
(itself at least 1), so the timeout-versus-expiry rule rejects it as well. The case "zero run expiry"
therefore passes through either rule, and removing the positive-integer check for this one key would
not fail a test; the value stays rejected.

The audit found four behaviours that differed from this file. Two are fixed in code and covered by the
tests above: F1, `budgets.tool_attempts`, which the catalog did not narrow, and F2,
`reports.enabled_templates`, which was not applied when queueing an existing report. The other two are
documented limits: F3 and F4.

### Known limitations

- **F3, allowed models are not probed.** `allowed_models` is checked per call. Neither the import nor
  the activation asks the provider whether a tag is installed, so an uninstalled tag activates and
  fails closed at the first model call.
- **F4, `model_input` runs on the judge's test entry.** The `model_input` boundary runs on
  `POST /api/control/evaluate`. Live agent runs inspect tool results and action proposals; nothing
  inspects the task context before it is sent to the model, so removing `model_input` from a guard's
  `boundaries` changes only what the test entry evaluates.
- `budgets.corrections` may be 0 in Go but not in the import, so an imported file cannot produce it.
- Lowering `budgets.corrections` or `budgets.calls_agent` through the catalog during a run has no test
  of its own; the `calls_total`, `calls_security`, token, timeout and tool-attempt ceilings do.

### Known limitations of the deterministic rules

A red team of the secret rules and the signature feed, as a judge's ad-hoc input would try them, is
pinned row by row in `services/gateway/internal/security/redteam_test.go`. The rules match normalized
substrings and keyword-anchored patterns. These inputs are **not** caught by a deterministic control; only
the semantic check could still object, and it is not complete detection:

- **Encodings are not decoded.** Base64, hex, URL-encoded keywords, HTML entities, literal `\u` escapes,
  ROT13 and reversed text pass.
- **Lookalike letters from other scripts are not folded.** A Cyrillic letter inside a keyword or a feed
  phrase passes. Compatibility forms (full-width, mathematical, circled and accented letters) are folded
  for signature matching only; the secret rules read the characters as written, except that they ignore
  invisible format characters.
- **Signature rules match one normalized phrase.** Paraphrase and leetspeak, words joined by hyphens,
  underscores or punctuation, letter-spaced text, a missing space, and wordings the feed does not list
  (`forget previous instructions`, `ignore previous directions`) pass. `feed_v2` lists the common wordings
  that end in `instructions`.
- **Secret keywords.** A keyword must be one word at the start of a word or after a non-alphanumeric
  character (`adminPassword` is missed), the value must follow it directly (`the password for the vault is
...` is missed), and the keyword list is `password`, `passwd`, `passcode`, `pwd`, `token`, `api key`,
  `secret` and `bearer` (`passphrase` is missed). A value that is only digits or only letters is not taken
  for a credential, and a vendor key without a keyword (`sk_live_...`) is not recognised.
- **A value split by whitespace** is masked only up to the split.
- **Account numbers.** An IBAN with hyphen, dot or newline separators, or glued to letters, and a card
  number with dots or newlines, glued to letters, in full-width digits or failing the Luhn check, are not
  masked. An IBAN also needs a registered country and its exact length.
- **Tool-result JSON.** Only string values are inspected, one at a time: a key name, a number, and text
  split across two values are not. Tool code, not the source record, names the keys.
- `secret_pattern` is not configured at `action_proposal` (it is not in its supported boundaries).

## Boundaries no setting can remove

Report: "Strictness changes optional detection behavior; they never remove tenant checks, immutable
task scope, approval integrity or inherited export restrictions. Model or budget increases cannot
silently widen an existing passport."

- The immutable passport is the upper bound on tools, records, templates, destinations, models and
  limits. A policy edit never changes a stored passport; broader authority needs a newly admitted run.
- Organization and resource checks run on every action regardless of the controls.
- Report provenance stays: an Internal only report cannot be exported to a vendor, whatever the
  controls, a classifier verdict or an approval says.
- Exact-action approval stays bound to the stored action; changed content, recipient or versions need
  a new proposal and review.
- A semantic verdict can restrict but never grant (AGENTS.md, guardrail 8): "a positive score would
  never authorize an otherwise forbidden tool, source, recipient, or report export."
- Disabling or loosening a guard never turns a deterministic denial into an allow, and a guard failure
  is never an allow.

## Rejected files

Each of these rejects the whole file and leaves the last accepted revision active:

| Case               | Example                                                                                   | Rule broken                                    |
| ------------------ | ----------------------------------------------------------------------------------------- | ---------------------------------------------- |
| Unknown control    | `controls.custom_plugin: {enabled: true}`                                                 | Only the three registered guard IDs            |
| Missing control    | `controls` without `signature_match`                                                      | All three guards are required                  |
| Unknown key        | `budgets.calls_bonus: 5`; `signature_match.mode: block`; `controls.secret_pattern.script` | Closed schema                                  |
| Unknown model      | `allowed_models: ["GPT 4"]`, `[]` or `["https://example.com/model"]`                      | Model tag syntax; non-empty list               |
| Impossible limit   | `calls_agent: 30` with `calls_total: 24`; `tokens_total: 0`; `corrections: 1.5`           | Positive integers; sub-limit ≤ total           |
| Impossible timeout | `request_timeout_seconds: 1200` with `run_expiry_minutes: 15`                             | Request deadline shorter than the run lifetime |
| URL or path        | `signatures.path: https://example.com/feed.json`, `../feed.json` or `a..json`             | Bare file name in this directory               |
| Out-of-range value | `threshold: 1.5`; `mode: allow`; `boundaries: [action_proposal]` on `secret_pattern`      | Allowed values per key                         |
| Wrong version      | `schema_version: 2`                                                                       | Version must be `1`                            |

A syntactically valid tag for a model that is not installed passes the import and the activation;
it fails closed at the first model call (see "Known limitations").
