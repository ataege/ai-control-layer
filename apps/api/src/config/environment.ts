import { z } from "zod";

const portSchema = z.coerce.number().int().min(1).max(65535);
// Same bounds as the Go gateway, so one .env value is valid for both services.
const timeoutMsSchema = z.coerce.number().int().min(100).max(20_000);

/** Variables needed to open a PostgreSQL connection (shared by the app and the TypeORM CLI). */
export const databaseEnvironmentSchema = z.object({
  POSTGRES_HOST: z.string().min(1),
  POSTGRES_PORT: portSchema.default(5432),
  POSTGRES_USER: z.string().min(1),
  POSTGRES_PASSWORD: z.string().min(1),
  POSTGRES_DB: z.string().min(1),
  DATABASE_TIMEOUT_MS: timeoutMsSchema.default(3000),
});

const httpUrlSchema = z.url({ protocol: /^https?$/ });

// Explicit origins only: "*" and anything with a path are rejected.
const corsOriginsSchema = z
  .string()
  .default("http://localhost:3000")
  .transform((commaSeparatedOrigins) =>
    commaSeparatedOrigins
      .split(",")
      .map((origin) => origin.trim())
      .filter((origin) => origin.length > 0),
  )
  .pipe(z.array(httpUrlSchema.refine((origin) => URL.parse(origin)?.origin === origin)).min(1));

/** Every variable the API reads. See the root .env.example. */
export const environmentSchema = databaseEnvironmentSchema.extend({
  NODE_ENV: z.enum(["development", "test", "production"]).default("development"),
  API_HOST: z.string().min(1).default("127.0.0.1"),
  API_PORT: portSchema.default(3001),
  GATEWAY_URL: httpUrlSchema,
  GATEWAY_SERVICE_TOKEN: z.string().min(32),
  GATEWAY_TIMEOUT_MS: timeoutMsSchema.default(3000),
  CORS_ALLOWED_ORIGINS: corsOriginsSchema,
  LOG_LEVEL: z.enum(["debug", "info", "warn", "error"]).default("info"),
});

export type DatabaseEnvironment = z.infer<typeof databaseEnvironmentSchema>;
export type Environment = z.infer<typeof environmentSchema>;

/** Thrown when variables are missing or invalid. The message names variables, never values. */
export class EnvironmentValidationError extends Error {
  constructor(
    readonly missingVariables: string[],
    readonly invalidVariables: string[],
  ) {
    const problems: string[] = [];
    if (missingVariables.length > 0) {
      problems.push(`missing: ${missingVariables.join(", ")}`);
    }
    if (invalidVariables.length > 0) {
      problems.push(`invalid: ${invalidVariables.join(", ")}`);
    }
    super(
      `Invalid environment configuration (${problems.join("; ")}). ` +
        "Run `pnpm run setup` at the repository root or see .env.example.",
    );
    this.name = "EnvironmentValidationError";
  }
}

type RawEnvironment = Record<string, string | undefined>;

/** Validates raw variables against a schema. Empty strings count as unset so defaults apply. */
export function parseEnvironment<Schema extends z.ZodType>(
  schema: Schema,
  rawEnvironment: RawEnvironment,
): z.infer<Schema> {
  const definedVariables = Object.fromEntries(
    Object.entries(rawEnvironment).filter(([, value]) => value !== undefined && value !== ""),
  );
  const result = schema.safeParse(definedVariables);
  if (result.success) {
    return result.data;
  }

  const failedVariables = new Set(result.error.issues.map((issue) => String(issue.path[0])));
  const missingVariables: string[] = [];
  const invalidVariables: string[] = [];
  for (const variableName of failedVariables) {
    const targetList = variableName in definedVariables ? invalidVariables : missingVariables;
    targetList.push(variableName);
  }
  throw new EnvironmentValidationError(missingVariables.sort(), invalidVariables.sort());
}
