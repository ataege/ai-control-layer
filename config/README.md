# Control policy configuration

`policy.yaml` in this directory is the documented, operator-editable sample of the Task Passport
control policy (roadmap task SH-42, interface X-78). Report version 1.2, "Central policy
configuration and safe reload": "The required sample configuration would be a documented
policy.yaml imported through NestJS into one central immutable control-catalog version."

This schema is the "policy activation and catalog revision" contract, owned by the web + API
implementer (`docs/product/README.md`, "Contracts to freeze first"). It is a draft until its owner
approves it and it merges into `main` through SH-11.

## How the file is used

| Step     | What happens                                                                                                                                                                                                                                                                             | Owner         |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| Edit     | An authorized policy owner or judge edits `policy.yaml`.                                                                                                                                                                                                                                 | operator      |
| Import   | An explicit command (API-32) or the authenticated reload (API-33) parses the whole file against the schema below, records its SHA-256 digest and stores it as a new immutable catalog revision. Nothing imports it at application startup.                                               | NestJS        |
| Validate | "Go fetches and validates the candidate, acknowledges readiness" (GO-73): Go validates the whole candidate against this shared schema, refuses one it cannot enforce, and also checks that each allowed model is available. The protocol is the open item `catalog activation protocol`. | Go            |
| Evaluate | Go reads the active revision before each new evaluation and dispatch and records the admission and evaluated revision in every decision (GO-72).                                                                                                                                         | Go            |
| Reject   | An invalid file never partially activates. The last accepted revision stays active and the rejection reason is visible (`policy_reload_rejected`). With no valid initial revision the gateway is not ready and dispatches nothing.                                                       | NestJS and Go |

The file is an import input, "not a second configuration authority": after an import, the stored
revision is what counts, and editing the file changes nothing until the next accepted import.

## Importing the file

From the repository root, with the database migrated (`pnpm db:migration:run`):

```sh
pnpm policy:import                 # imports config/policy.yaml
pnpm policy:import path/to/copy.yaml
```

The command (API-32) validates the whole file and then, in one transaction:

- **Valid file:** stores a new immutable revision (`app.control_catalog_revisions`) and makes it the
  requested revision. The very first accepted revision also becomes active. A later revision stays
  requested until the authenticated reload (API-33) activates it after Go acknowledges it
  (`catalog activation protocol`). Exit code 0.
- **Invalid file:** stores nothing. It records the reason (`policy_reload_rejected`), the file
  digest and up to 20 issues on the pointer (`app.control_catalog_pointer`); the active revision
  stays. Exit code 1.

Before the schema is checked, the file must be at most 64 KiB of valid UTF-8 without control
characters (tab, line feed and carriage return are allowed) and hold one plain YAML document: no
duplicate keys, no aliases and no custom tags. A leading byte order mark is kept in the stored text, so
the stored text always matches the stored digest. A relative path is relative to the repository root.
Rejection messages are fixed texts per kind of problem; they never repeat a key or value from the
file. The command never runs at application
startup, and it does not bind a signature feed: the feed import (API-34) does that.

## Values

Every number in the sample is an illustrative value from the project report, labelled there as
"illustrative team settings, not sponsor requirements or measured performance". The team fixes the
real values at the M0 freeze (X-06). Limits "depend on the selected model and hardware and require
measurement". The model `qwen3.5:4b` is provisional: decision 6 is open, and the probe that chose it
is in `docs/setup.md`, section 7.

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
has a default: every key in the tables is required.

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
model runs, so Go checks it when it validates the candidate. A request for a model outside the active
list is denied before dispatch (`model_not_allowed`), even on a previously admitted run.

`budgets`, all positive integers written in plain decimal digits (zero, negative or fractional values, and spellings such as `24.0`, `0x18` or `0o30`, reject the file):

| Key                       | Sample | Meaning                                                                                 |
| ------------------------- | ------ | --------------------------------------------------------------------------------------- |
| `calls_total`             | 24     | Model calls per run across both purposes.                                               |
| `calls_agent`             | 12     | Ceiling for agent-purpose calls. Must not exceed `calls_total`.                         |
| `calls_security`          | 12     | Ceiling for security-purpose (semantic check) calls. Must not exceed `calls_total`.     |
| `tokens_total`            | 20000  | Input and output tokens per run across both purposes.                                   |
| `request_timeout_seconds` | 20     | Deadline for one model request. Must be shorter than `run_expiry_minutes` (in seconds). |
| `local_max_concurrency`   | 2      | Concurrent local model requests.                                                        |
| `run_expiry_minutes`      | 15     | Run lifetime from admission.                                                            |
| `tool_attempts`           | 12     | Governed tool attempts per run, safe retries included.                                  |
| `corrections`             | 2      | Bounded feedback rounds after a denied proposal.                                        |

The purpose sub-limits are ceilings, "not reserved entitlements": "Both purposes count toward the same
totals." Each sub-limit must be less than or equal to `calls_total`; their sum may exceed it. A call is
reserved against both the shared ceiling and its purpose ceiling before dispatch, so security calls
"cannot escape the task ceiling".

Budget changes apply as current restrictions. Lowering a budget constrains future dispatches of
running tasks. Raising one never exceeds the ceiling stored in an admitted passport; "an expanded
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
| `threshold`  | number          | The semantic risk cutoff. Provisionally from 0 to 1 inclusive, the score range the SH-45 probe asked the model for. See the note below.      |
| `boundaries` | list of strings | Where the guard runs. Non-empty, no duplicates, each one of the values the guard supports.                                                   |

The boundaries are the report's "Proposed hybrid evaluation boundaries": before model dispatch
(`model_input`), tool result to agent context (`tool_result`) and the proposed action
(`action_proposal`).

Notes on each guard:

- `secret_pattern`: covers the report's "Configured secret/PII patterns". Where the patterns are
  defined, which fields they apply to and how match spans are validated is the open item
  `redaction rules`; this schema carries no pattern or regular expression.
- `semantic_injection`: the 0.75 cutoff "is an illustrative score cutoff, not a calibrated
  probability". Go validates the verdict's schema and "applies policy thresholds itself". The score
  range and the comparison (at or above, or above) belong to the open item `classifier prompt and
verdict schema`, so the 0 to 1 range here is provisional and changes with it. Semantic suspicion
  in `redact` mode "may cause the server to mask the whole configured field rather than accepting
  free-form rewritten text". At `action_proposal`, "Allow, deny and require approval remain the
  principal action outcomes; redaction applies to supported content fields", so what `redact` means
  for a proposal belongs to `redaction rules`. The security request is metered against
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

The issuer, its key and the feed digest are properties of the feed file and its import, not of this
policy. The feed file (`attack-signatures.json`, SH-46) does not exist yet, so the policy import
(API-32) checks the `signatures` keys syntactically only. Validating the feed's content, issuer and
signature, size and pattern grammar, and checking that each `disabled_rules` ID exists in it, is the
feed import (API-34, blocked by `feed grammar and trust`).

The report says "The same activation command validates the referenced versioned attack-signatures.json
feed. A failed validation leaves the accepted active version intact". The recommendation for the open
item `catalog activation protocol`: a candidate whose enabled `signature_match` references a feed
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
is denied (`template_not_allowed`). A template listed here still needs the passport to allow it.

The other settings the report names for this group are not in the schema yet, and the schema holds no
values for them:

- Projections: the template and projection-rule records are `app` records (SH-41); what the vendor
  projection holds is the open item `vendor projection fields`.
- Authorized destinations: where recipient rules live is the open item `source classification
storage`.
- Safe export fields: set by the security summary and audit export record contract (web + API
  implementer). JSON and CSV export are fixed capabilities, not settings.

Go derives every classification from trusted records; no setting here classifies or declassifies a
report.

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

A syntactically valid tag for a model that is not installed passes the import and is rejected when
Go validates the candidate.
