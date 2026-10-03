// Entry point for the TypeORM CLI only (see scripts/typeorm-cli.mjs). The Nest app never imports it.
import "reflect-metadata";
import { DataSource } from "typeorm";
import { databaseEnvironmentSchema, parseEnvironment } from "../config/environment.js";
import { buildTypeOrmOptions } from "./typeorm-options.js";

// The CLI does not load .env files: POSTGRES_* must already be in the environment.
const databaseEnvironment = parseEnvironment(databaseEnvironmentSchema, process.env);

export default new DataSource(buildTypeOrmOptions(databaseEnvironment));
