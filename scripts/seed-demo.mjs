#!/usr/bin/env node
// Demo seed (`pnpm db:seed`): an explicit command, never run at startup (lead decision: the seed
// data lives in fixtures/, the command is this file). DRAFT: it writes the unapproved SH-17 tables.
//
// 1. The same checks as reset:demo: a loopback database that answers.
// 2. The synthetic vendors and invoices from fixtures/demo-records.json, in one transaction.
//    Idempotent: missing rows are inserted, present rows are left alone; a present row with other
//    values stops the seed and changes nothing.
// 3. The control catalog through `pnpm policy:import` (API-32, Worker 1), only when that script
//    exists on this branch; otherwise the step is reported as skipped.
//
// TODO: the app records (organizations, users, memberships) are not seeded here. They belong to
// the identity tables (API-05 to API-08), and the demonstration operator (SH-19) needs their
// password hashing; add them when those land.
import { readFileSync } from "node:fs";

import { runCommand } from "./lib/commands.mjs";
import { requireLocalReachableDatabase } from "./lib/database-probe.mjs";
import {
  connectToDatabase,
  countRows,
  listTables,
  requireDemoTables,
  seedDemoRecords,
} from "./lib/demo-seed.mjs";
import { loadRootEnvironment, MISSING_ENV_FILE_MESSAGE } from "./lib/env-file.mjs";
import { printStatus } from "./lib/output.mjs";
import { fromRepositoryRoot, repositoryRoot } from "./lib/repo-root.mjs";

const POLICY_IMPORT_SCRIPT = "policy:import";

const { fileFound, environment } = loadRootEnvironment();
if (!fileFound) {
  console.error(MISSING_ENV_FILE_MESSAGE);
  process.exit(1);
}
if (!(await requireLocalReachableDatabase(environment, printStatus, "seed"))) process.exit(1);

const client = await connectToDatabase(environment);
try {
  await requireDemoTables(client);
  const demoTables = await listTables(client, ["demo"]);
  printStatus("info", `before: ${JSON.stringify(await countRows(client, demoTables))}`);

  await client.query("BEGIN");
  try {
    const seeded = await seedDemoRecords(client);
    await client.query("COMMIT");
    for (const [tableName, result] of Object.entries(seeded)) {
      printStatus(
        "ok",
        `demo.${tableName}: ${result.inserted} inserted, ${result.alreadyPresent} already present`,
      );
    }
  } catch (seedError) {
    await client.query("ROLLBACK");
    throw seedError;
  }
  printStatus("info", `after: ${JSON.stringify(await countRows(client, demoTables))}`);
} catch (seedError) {
  printStatus("fail", `seed failed: ${seedError.message}`);
  process.exit(1);
} finally {
  await client.end();
}

printStatus(
  "info",
  "app records (organizations, users, memberships) not seeded: TODO with API-05 to API-08 and SH-19",
);

const rootScripts =
  JSON.parse(readFileSync(fromRepositoryRoot("package.json"), "utf8")).scripts ?? {};
if (!(POLICY_IMPORT_SCRIPT in rootScripts)) {
  printStatus(
    "warn",
    `control catalog skipped: no \`${POLICY_IMPORT_SCRIPT}\` script on this branch (API-32)`,
  );
  process.exit(0);
}
const importExitCode = await runCommand("pnpm", ["run", POLICY_IMPORT_SCRIPT], {
  cwd: repositoryRoot,
});
if (importExitCode !== 0) {
  printStatus("fail", `control catalog import failed (exit code ${importExitCode})`);
  process.exit(1);
}
printStatus("ok", "control catalog imported");
