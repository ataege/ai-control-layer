#!/usr/bin/env node
// Demo reset (`pnpm reset:demo`, `make reset-demo`): an explicit command for the judge environment,
// never run at startup. Decided scope (lead, 2026-10-03, SH-29 option b): truncate the demo and
// runtime data and reseed the synthetic demo records; app data (users, memberships, control-catalog
// revisions) is kept, so a judge's policy edits survive a fixture reset. It never removes the
// database volume. DRAFT: it works on the unapproved SH-16, SH-17, SH-24, SH-27 and SH-44 tables.
//
// The truncate and the reseed run in one transaction: either the fixtures are fully restored or
// nothing changed. It imports config/policy.yaml only into an empty catalog (a fresh database); it
// never re-imports over an existing one, which would replace a judge's active catalog revision.
import { runCommand } from "./lib/commands.mjs";
import { requireLocalReachableDatabase } from "./lib/database-probe.mjs";
import { GATEWAY_ROLE, setGatewayRolePassword } from "./lib/database-roles.mjs";
import {
  connectToDatabase,
  countRows,
  listTables,
  requireDemoTables,
  seedDemoRecords,
} from "./lib/demo-seed.mjs";
import { loadRootEnvironment, MISSING_ENV_FILE_MESSAGE } from "./lib/env-file.mjs";
import { printStatus } from "./lib/output.mjs";
import { repositoryRoot } from "./lib/repo-root.mjs";

const RESET_SCHEMAS = ["demo", "runtime"];

const { fileFound, environment } = loadRootEnvironment();
if (!fileFound) {
  console.error(MISSING_ENV_FILE_MESSAGE);
  process.exit(1);
}
// Only a database on this machine: the reset deletes data and must not reach a shared one.
if (!(await requireLocalReachableDatabase(environment, printStatus, "reset"))) process.exit(1);

const client = await connectToDatabase(environment);
let catalogIsEmpty = false;
try {
  await requireDemoTables(client);
  const resetTables = await listTables(client, RESET_SCHEMAS);
  printStatus("info", `before: ${JSON.stringify(await countRows(client, resetTables))}`);

  await client.query("BEGIN");
  try {
    // One statement over every table of both schemas, so their foreign keys need no CASCADE;
    // a table outside them that still references one makes the reset fail instead.
    await client.query(`TRUNCATE ${resetTables.join(", ")} RESTART IDENTITY`);
    const seeded = await seedDemoRecords(client);
    await client.query("COMMIT");
    printStatus("ok", `truncated ${resetTables.length} tables in ${RESET_SCHEMAS.join(" and ")}`);
    for (const [tableName, result] of Object.entries(seeded)) {
      printStatus("ok", `demo.${tableName}: ${result.inserted} reseeded`);
    }
  } catch (resetError) {
    await client.query("ROLLBACK");
    throw resetError;
  }
  printStatus("info", `after: ${JSON.stringify(await countRows(client, resetTables))}`);
  printStatus("info", "app data (identity, control catalog) kept");
  // The same step as `pnpm db:roles`, so the presentation flow cannot miss it.
  await setGatewayRolePassword(client, environment);
  printStatus("ok", `${GATEWAY_ROLE}: login password set (as pnpm db:roles)`);
  // A fresh database has no catalog revision yet; import config/policy.yaml once (as db:seed
  // does), so the activation below has something to activate. An existing catalog is never replaced.
  const { rows } = await client.query(
    "SELECT count(*)::int AS revisions FROM app.control_catalog_revisions",
  );
  catalogIsEmpty = rows[0].revisions === 0;
} catch (resetError) {
  printStatus("fail", `reset failed, nothing changed: ${resetError.message}`);
  process.exit(1);
} finally {
  await client.end();
}

// An existing catalog is kept and never re-imported; this runs the gateway's own activation once, so a
// requested revision is validated and the demo is enforceable before the gateway starts (the
// running gateway's watcher remains the normal path). It fails when no catalog is active.
if (catalogIsEmpty) {
  const importExitCode = await runCommand("pnpm", ["run", "policy:import"], {
    cwd: repositoryRoot,
    env: environment,
  });
  if (importExitCode !== 0) {
    printStatus(
      "fail",
      `the data was reset, but importing config/policy.yaml failed (exit ${importExitCode})`,
    );
    process.exit(1);
  }
  printStatus("ok", "control catalog was empty: config/policy.yaml imported (pnpm policy:import)");
}

const activationExitCode = await runCommand("pnpm", ["run", "catalog:activate"], {
  cwd: repositoryRoot,
  env: environment,
});
if (activationExitCode !== 0) {
  printStatus(
    "fail",
    `the data was reset, but no enforceable control catalog (exit ${activationExitCode})`,
  );
  process.exit(1);
}
printStatus("ok", "control catalog active (pnpm catalog:activate)");
