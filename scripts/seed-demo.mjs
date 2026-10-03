#!/usr/bin/env node
// Demo seed (`pnpm db:seed`): an explicit command, never run at startup (lead decision: the seed
// data lives in fixtures/, the command is this file). DRAFT: it writes the unapproved SH-17 tables.
//
// 1. The same checks as reset:demo: a loopback database that answers.
// 2. The synthetic vendors and invoices from fixtures/demo-records.json, in one transaction.
//    Idempotent: missing rows are inserted, present rows are left alone; a present row with other
//    values stops the seed and changes nothing.
// 3. The control catalog through `pnpm policy:import` (API-32, Worker 1), only when that script
//    exists and the catalog is still empty; otherwise the step is reported as skipped, so a second
//    run adds no duplicate revision and never replaces a judge's later policy edits.
//
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

// Run the API-owned credential seed through its existing TypeScript loader. No startup hook.
const operatorSeedExitCode = await runCommand(
  process.execPath,
  [
    "--loader",
    fromRepositoryRoot("apps/api/node_modules/ts-node/esm.mjs"),
    "--no-warnings",
    fromRepositoryRoot("apps/api/src/auth/seed-demo-operator.command.ts"),
  ],
  {
    cwd: repositoryRoot,
    env: { ...environment, TS_NODE_PROJECT: fromRepositoryRoot("apps/api/tsconfig.json") },
  },
);
if (operatorSeedExitCode !== 0) process.exit(operatorSeedExitCode);

const client = await connectToDatabase(environment);
// The catalog state before the import step: the seed imports policy.yaml only into an empty
// catalog, so a second run adds no duplicate revision and never replaces a judge's later edits.
let catalogTablesPresent = false;
let existingCatalogRevisionCount = 0;
let activeCatalogRevisionId = null;
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

  const catalogPresence = await client.query(
    `SELECT to_regclass('app.control_catalog_revisions') IS NOT NULL
        AND to_regclass('app.control_catalog_pointer') IS NOT NULL AS present`,
  );
  catalogTablesPresent = catalogPresence.rows[0].present;
  if (catalogTablesPresent) {
    const catalogState = await client.query(
      `SELECT (SELECT count(*)::int FROM app.control_catalog_revisions) AS revision_count,
              (SELECT active_revision_id::text FROM app.control_catalog_pointer WHERE id = 1) AS active_revision_id`,
    );
    existingCatalogRevisionCount = catalogState.rows[0].revision_count;
    activeCatalogRevisionId = catalogState.rows[0].active_revision_id;
  }
} catch (seedError) {
  printStatus("fail", `seed failed: ${seedError.message}`);
  process.exit(1);
} finally {
  await client.end();
}

const rootScripts =
  JSON.parse(readFileSync(fromRepositoryRoot("package.json"), "utf8")).scripts ?? {};
if (!(POLICY_IMPORT_SCRIPT in rootScripts)) {
  printStatus(
    "warn",
    `control catalog skipped: no \`${POLICY_IMPORT_SCRIPT}\` script on this branch (API-32)`,
  );
  process.exit(0);
}
if (!catalogTablesPresent) {
  printStatus("warn", "control catalog skipped: the app catalog tables are not migrated");
  process.exit(0);
}
if (existingCatalogRevisionCount > 0) {
  printStatus(
    "ok",
    `control catalog already seeded (${existingCatalogRevisionCount} revisions, active ${activeCatalogRevisionId ?? "none"}); not re-imported`,
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
