import type { DatabaseEnvironment, Environment } from "./environment.js";

/** Typed, read-only view of the validated environment. */
export class AppConfigService {
  constructor(private readonly environment: Environment) {}

  get nodeEnvironment(): Environment["NODE_ENV"] {
    return this.environment.NODE_ENV;
  }

  get apiHost(): string {
    return this.environment.API_HOST;
  }

  get apiPort(): number {
    return this.environment.API_PORT;
  }

  get corsAllowedOrigins(): string[] {
    return this.environment.CORS_ALLOWED_ORIGINS;
  }

  get gatewayUrl(): string {
    return this.environment.GATEWAY_URL;
  }

  /** Secret. Only the gateway client may read it; never log or return it. */
  get gatewayServiceToken(): string {
    return this.environment.GATEWAY_SERVICE_TOKEN;
  }

  get gatewayTimeoutMs(): number {
    return this.environment.GATEWAY_TIMEOUT_MS;
  }

  get cookieSecret(): string {
    return this.environment.COOKIE_SECRET;
  }

  get databaseTimeoutMs(): number {
    return this.environment.DATABASE_TIMEOUT_MS;
  }

  get logLevel(): Environment["LOG_LEVEL"] {
    return this.environment.LOG_LEVEL;
  }

  /** Connection settings handed to the shared TypeORM options factory. */
  get database(): DatabaseEnvironment {
    return this.environment;
  }
}
