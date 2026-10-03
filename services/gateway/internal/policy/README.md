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

| Tool            | Arguments                                                                     |
| --------------- | ----------------------------------------------------------------------------- |
| `read_invoice`  | `invoice_id`                                                                  |
| `read_vendor`   | `vendor_id`                                                                   |
| `create_report` | `template` (one of the two fixed templates), `invoice_ids` (ordered, unique)  |
| `queue_report`  | `report_id` (lowercase UUID), `recipient_reference` (trusted directory entry) |
