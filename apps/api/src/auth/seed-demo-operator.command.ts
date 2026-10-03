import "reflect-metadata";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { DataSource } from "typeorm";
import { buildTypeOrmOptions } from "../database/typeorm-options.js";
import { databaseEnvironmentSchema, parseEnvironment } from "../config/environment.js";
import { seedDemoOperator } from "./demo-operator-seed.js";

const records = JSON.parse(readFileSync(resolve("fixtures/demo-records.json"), "utf8")) as {
  organizations: { key: string; id: string; name: string }[];
  task_scope: { organization_id: string };
};
const organization = records.organizations.find((record) => record.key === "demo_org");
if (!organization || organization.id !== records.task_scope.organization_id) {
  throw new Error("The development demonstration organization does not match the task fixture");
}
const dataSource = new DataSource(
  buildTypeOrmOptions(parseEnvironment(databaseEnvironmentSchema, process.env)),
);
try {
  await dataSource.initialize();
  await seedDemoOperator(dataSource.manager, organization, process.env.DEMO_OPERATOR_PASSWORD);
  console.log(
    "Development Demonstration: organization, operator and operator/reviewer membership seeded (existing records preserved)",
  );
} catch {
  console.error(
    "Development demonstration seed failed; check migrations, seed identity and DEMO_OPERATOR_PASSWORD. No credentials were printed.",
  );
  process.exitCode = 1;
} finally {
  if (dataSource.isInitialized) await dataSource.destroy();
}
