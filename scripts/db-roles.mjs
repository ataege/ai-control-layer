#!/usr/bin/env node
// `pnpm db:roles`: gives the gateway's database role (task_passport_gateway) its login password
// from POSTGRES_GATEWAY_PASSWORD, after `pnpm db:migration:run` (GO-38). Explicit and idempotent;
// runs as the owner from .env and never at startup. `pnpm reset:demo` runs the same step.
// Container mode: run it from the host against the published PostgreSQL port, like the migrations.
import { requireLocalReachableDatabase } from "./lib/database-probe.mjs";
import { GATEWAY_ROLE, setGatewayRolePassword } from "./lib/database-roles.mjs";
import { connectToDatabase } from "./lib/demo-seed.mjs";
import { loadRootEnvironment, MISSING_ENV_FILE_MESSAGE } from "./lib/env-file.mjs";
import { printStatus } from "./lib/output.mjs";

const { fileFound, environment } = loadRootEnvironment();
if (!fileFound) {
  console.error(MISSING_ENV_FILE_MESSAGE);
  process.exit(1);
}
if (!(await requireLocalReachableDatabase(environment, printStatus, "set the roles of")))
  process.exit(1);

const client = await connectToDatabase(environment);
try {
  await setGatewayRolePassword(client, environment);
  printStatus(
    "ok",
    `${GATEWAY_ROLE}: login enabled with the password from POSTGRES_GATEWAY_PASSWORD`,
  );
} catch (rolesError) {
  printStatus("fail", `db:roles failed: ${rolesError.message}`);
  process.exit(1);
} finally {
  await client.end();
}
