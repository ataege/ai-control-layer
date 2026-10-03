import type { EntityManager } from "typeorm";
import { hashPassword, verifyPassword } from "./password.util.js";

export const DEMO_OPERATOR_EMAIL = "demo-operator@example.com";
export const DEMO_OPERATOR_ID = "c28e2545-2de6-41b9-9be6-d21ae01e4901";
const demonstrationName = "Development Demonstration Operator";
const demonstrationRoles = ["operator", "reviewer"];

/** Explicit seed only. Existing identities, credentials and authority are never overwritten. */
export async function seedDemoOperator(
  manager: EntityManager,
  organization: { id: string; name: string },
  password: string | undefined,
): Promise<void> {
  if (!password || password.length > 256) {
    throw new Error("Set DEMO_OPERATOR_PASSWORD in .env (1 to 256 characters) before seeding");
  }
  await manager.transaction(async (transaction) => {
    await transaction.query("SELECT pg_advisory_xact_lock(1791190000)");
    await transaction.query(
      `INSERT INTO app.organizations (id, name) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`,
      [organization.id, organization.name],
    );
    await transaction.query(
      `INSERT INTO app.users (id, email, name) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`,
      [DEMO_OPERATOR_ID, DEMO_OPERATOR_EMAIL, demonstrationName],
    );
    const [storedOrganization] = await transaction.query<{ name: string }[]>(
      "SELECT name FROM app.organizations WHERE id = $1",
      [organization.id],
    );
    const [user] = await transaction.query<{ email: string; name: string }[]>(
      "SELECT email, name FROM app.users WHERE id = $1",
      [DEMO_OPERATOR_ID],
    );
    if (
      storedOrganization?.name !== organization.name ||
      user?.email !== DEMO_OPERATOR_EMAIL ||
      user.name !== demonstrationName
    ) {
      throw new Error(
        "Development demonstration identity differs from the seed; no identity was overwritten",
      );
    }
    const [credential] = await transaction.query<{ hash: string }[]>(
      `SELECT hash FROM app.password_hashes WHERE "userId" = $1`,
      [DEMO_OPERATOR_ID],
    );
    if (credential) {
      if (!(await verifyPassword(password, credential.hash))) {
        throw new Error(
          "Development demonstration password differs from .env; no credential was overwritten",
        );
      }
    } else {
      await transaction.query(`INSERT INTO app.password_hashes ("userId", hash) VALUES ($1, $2)`, [
        DEMO_OPERATOR_ID,
        await hashPassword(password),
      ]);
    }
    await transaction.query(
      `INSERT INTO app.memberships ("userId", "organizationId", roles)
       SELECT $1, $2, $3::text[] WHERE NOT EXISTS
       (SELECT 1 FROM app.memberships WHERE "userId" = $1)`,
      [DEMO_OPERATOR_ID, organization.id, demonstrationRoles],
    );
    const memberships = await transaction.query<{ organizationId: string; roles: string[] }[]>(
      `SELECT "organizationId", roles FROM app.memberships WHERE "userId" = $1`,
      [DEMO_OPERATOR_ID],
    );
    if (
      memberships.length !== 1 ||
      memberships[0]?.organizationId !== organization.id ||
      JSON.stringify([...memberships[0].roles].sort()) !==
        JSON.stringify([...demonstrationRoles].sort())
    ) {
      throw new Error(
        "Development demonstration membership differs from the seed; no authority was overwritten",
      );
    }
  });
}
