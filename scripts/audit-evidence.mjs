#!/usr/bin/env node
// Evidence audit (SH-32): checks files before they are committed to docs/evidence/.
//   node scripts/audit-evidence.mjs [--root <dir>] <file>...
// A file fails when it holds any of:
// - the value of a `.env` entry whose name contains PASSWORD, SECRET, TOKEN, KEY or SIGNING
//   (8 characters or more);
// - an absolute path of this machine (/Users/, /home/, /private/, /var/folders, a drive letter, or
//   the repository root); a bare /tmp/... inside a quoted example, such as a test-case title, is
//   not a path of ours and passes;
// - a URL other than example.*, localhost or 127.0.0.1, a Bearer value, a JWT or a session= value;
// - the first 30 characters of a corpus text, hostile note, internal note or registered address
//   from fixtures/ (prompt and record text never belongs in evidence);
// - invalid JSON, for a `.json` file.
// It prints names and short finding labels only, never a secret value, and exits 1 when any file
// fails. `--root` points at another repository root (used by the test with a temporary one).
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { readEnvFile } from "./lib/env-file.mjs";
import { repositoryRoot } from "./lib/repo-root.mjs";

const SECRET_NAME_PATTERN = /(PASSWORD|SECRET|TOKEN|KEY|SIGNING)/;
const MINIMUM_SECRET_LENGTH = 8;
const FRAGMENT_LENGTH = 30;
const MINIMUM_TEXT_LENGTH = 12;

const PATTERN_RULES = [
  ["path", /(\/Users\/|\/home\/|\/private\/|\/var\/folders|[A-Za-z]:\\)/],
  ["url", /https?:\/\/(?!example\.|localhost|127\.0\.0\.1)/],
  ["token", /(Bearer\s+\S+|eyJ[A-Za-z0-9_-]{10,}|session=[A-Za-z0-9-]{8,})/],
];

function readJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

/** What the audit looks for, read from the root's `.env` and `fixtures/`. */
export function loadAuditContext(root = repositoryRoot) {
  const environment = readEnvFile(join(root, ".env")) ?? {};
  const secrets = Object.entries(environment).filter(
    ([name, value]) => SECRET_NAME_PATTERN.test(name) && value.length >= MINIMUM_SECRET_LENGTH,
  );
  const corpusCases = readJson(join(root, "fixtures", "semantic-corpus.json")).cases ?? [];
  const hostileNotes = readJson(join(root, "fixtures", "hostile-notes.json")).notes ?? [];
  const records = readJson(join(root, "fixtures", "demo-records.json"));
  const texts = [
    ...corpusCases.map((entry) => entry.text),
    ...hostileNotes.map((entry) => entry.text),
    ...records.invoices.map((invoice) => invoice.internal_note),
    ...records.vendors.map((vendor) => vendor.registered_reporting_address),
  ];
  const fragments = texts
    .filter((text) => typeof text === "string" && text.length >= MINIMUM_TEXT_LENGTH)
    .map((text) => text.slice(0, FRAGMENT_LENGTH));
  return { secrets, fragments, root };
}

/** Findings for one file's text; each is a label that carries no secret value. */
export function auditText(text, { isJson, context }) {
  const findings = [];
  for (const [name, value] of context.secrets) {
    if (text.includes(value)) findings.push(`secret value of ${name}`);
  }
  if (context.root && text.includes(context.root)) findings.push("repository root path");
  for (const [label, pattern] of PATTERN_RULES) {
    const match = text.match(pattern);
    if (match) findings.push(`${label}: ${match[0].slice(0, 24)}`);
  }
  context.fragments.forEach((fragment, index) => {
    if (text.includes(fragment)) findings.push(`fixture or note text #${index}`);
  });
  if (isJson) {
    try {
      JSON.parse(text);
    } catch {
      findings.push("not valid JSON");
    }
  }
  return findings;
}

/** Audits the files and returns one result per file. */
export function auditFiles(files, context) {
  return files.map((file) => {
    const findings = auditText(readFileSync(file, "utf8"), {
      isJson: file.endsWith(".json"),
      context,
    });
    return { file, findings };
  });
}

function main(argumentsList) {
  let root = repositoryRoot;
  const files = [];
  for (let index = 0; index < argumentsList.length; index += 1) {
    if (argumentsList[index] === "--root") {
      root = argumentsList[(index += 1)];
    } else {
      files.push(argumentsList[index]);
    }
  }
  if (files.length === 0) {
    console.error("Usage: node scripts/audit-evidence.mjs [--root <dir>] <file>...");
    return 2;
  }
  const context = loadAuditContext(root);
  const results = auditFiles(files, context);
  for (const { file, findings } of results) {
    console.log(
      `${findings.length === 0 ? "PASS" : "FAIL"} ${file}${findings.length === 0 ? "" : ` -> ${findings.join("; ")}`}`,
    );
  }
  console.log(
    `secrets checked: ${context.secrets.length} names; fixture fragments: ${context.fragments.length}`,
  );
  return results.some((result) => result.findings.length > 0) ? 1 : 0;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  process.exit(main(process.argv.slice(2)));
}
