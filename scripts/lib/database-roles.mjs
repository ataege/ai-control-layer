// Gives the gateway's database role its login password (GO-38). Shared by `pnpm db:roles` and
// `pnpm reset:demo`. Runs as the owner (POSTGRES_USER), never at startup; idempotent.

// The role CreateServiceRoles creates; its name is fixed and never configurable.
export const GATEWAY_ROLE = "task_passport_gateway";

/**
 * Sets LOGIN and the password from POSTGRES_GATEWAY_PASSWORD on the gateway role. Fails with the
 * fix when the variable is missing or the role does not exist yet. The password is never printed.
 */
export async function setGatewayRolePassword(client, environment) {
  const password = environment.POSTGRES_GATEWAY_PASSWORD;
  if (!password || password.trim() === "") {
    throw new Error("POSTGRES_GATEWAY_PASSWORD is not set; run `pnpm run setup` first");
  }
  const role = await client.query(`SELECT 1 FROM pg_roles WHERE rolname = $1`, [GATEWAY_ROLE]);
  if (role.rowCount !== 1) {
    throw new Error(`the role ${GATEWAY_ROLE} does not exist; run \`pnpm db:migration:run\` first`);
  }
  // ALTER ROLE takes no bind parameters: format() quotes the name and the password literal safely.
  const statement = await client.query(
    `SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L', $1::text, $2::text) AS sql`,
    [GATEWAY_ROLE, password],
  );
  await client.query(statement.rows[0].sql);
}
