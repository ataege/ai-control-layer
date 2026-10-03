#!/usr/bin/env node
// Demo reset (`pnpm reset:demo`, `make reset-demo`): an explicit command for the judge environment,
// never run at startup. Decided scope (lead, 2026-10-03, SH-29 option b): truncate the demo and
// runtime data and reseed the synthetic demo records; app data (users, memberships, control-catalog
// revisions) is kept, so a judge's policy edits survive a fixture reset. It never removes the
// database volume.
//
// Part 1 (this file today): the safety checks. The reset itself lands with the demo and runtime
// tables and the seed command (SH-17, SH-24, SH-18); until then the command stops with an error
// after the checks and changes nothing, so it can never be mistaken for a completed reset.
import { loadRootEnvironment, MISSING_ENV_FILE_MESSAGE } from "./lib/env-file.mjs";
import { databaseAddress, databaseIsReachable, hostIsLoopback } from "./lib/database-probe.mjs";
import { printStatus } from "./lib/output.mjs";

const { fileFound, environment } = loadRootEnvironment();
if (!fileFound) {
  console.error(MISSING_ENV_FILE_MESSAGE);
  process.exit(1);
}

const database = databaseAddress(environment);

// 1. Only a database on this machine: the reset deletes data and must not reach a shared one.
if (!(await hostIsLoopback(database.host))) {
  printStatus(
    "fail",
    `refusing to reset ${database.label}: POSTGRES_HOST must resolve only to a loopback address (localhost, 127.0.0.1, ::1)`,
  );
  process.exit(1);
}
printStatus("ok", `${database.host} is a loopback address`);

// 2. The database must answer before anything is changed.
if (!(await databaseIsReachable(database))) {
  printStatus(
    "fail",
    `cannot connect to ${database.label}; start PostgreSQL (pnpm infra:up) or fix POSTGRES_HOST/POSTGRES_PORT`,
  );
  process.exit(1);
}
printStatus("ok", `${database.label} reachable`);

// 3. Not built yet: nothing was reset.
printStatus(
  "fail",
  "the reset is not implemented yet: it needs the demo and runtime tables and the seed command (SH-17, SH-24, SH-18). Nothing was changed.",
);
process.exit(1);
