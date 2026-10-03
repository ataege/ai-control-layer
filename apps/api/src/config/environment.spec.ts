import { describe, expect, it } from "vitest";
import { EnvironmentValidationError, environmentSchema, parseEnvironment } from "./environment.js";

const validEnvironment = {
  POSTGRES_HOST: "localhost",
  POSTGRES_USER: "starter",
  POSTGRES_PASSWORD: "example-database-password",
  POSTGRES_DB: "starter",
  GATEWAY_URL: "http://localhost:8080",
  GATEWAY_SERVICE_TOKEN: "example-service-token-0123456789abcdef",
  OPERATOR_CONTEXT_SIGNING_KEY: "test-signing-key-0123456789abcdef",
};

function captureValidationError(
  rawEnvironment: Record<string, string>,
): EnvironmentValidationError {
  try {
    parseEnvironment(environmentSchema, rawEnvironment);
  } catch (error) {
    if (error instanceof EnvironmentValidationError) {
      return error;
    }
  }
  throw new Error("expected parseEnvironment to throw EnvironmentValidationError");
}

describe("parseEnvironment", () => {
  it("applies defaults and coerces numbers and the origin list", () => {
    const environment = parseEnvironment(environmentSchema, {
      ...validEnvironment,
      API_PORT: "3210",
      // Empty values (as in .env.example) fall back to the default.
      GATEWAY_TIMEOUT_MS: "",
      CORS_ALLOWED_ORIGINS: "http://localhost:3000, https://app.example.test",
    });

    expect(environment.API_PORT).toBe(3210);
    expect(environment.API_HOST).toBe("127.0.0.1");
    expect(environment.GATEWAY_TIMEOUT_MS).toBe(3000);
    expect(environment.DATABASE_TIMEOUT_MS).toBe(3000);
    expect(environment.CORS_ALLOWED_ORIGINS).toEqual([
      "http://localhost:3000",
      "https://app.example.test",
    ]);
  });

  it("reports missing and invalid variables by name without echoing any value", () => {
    const error = captureValidationError({
      POSTGRES_HOST: "localhost",
      POSTGRES_USER: "starter",
      POSTGRES_PASSWORD: "",
      POSTGRES_DB: "starter",
      GATEWAY_URL: "not-a-url-value",
      GATEWAY_SERVICE_TOKEN: "too-short-token-value",
    });

    expect(error.missingVariables).toEqual(["OPERATOR_CONTEXT_SIGNING_KEY", "POSTGRES_PASSWORD"]);
    expect(error.invalidVariables).toEqual(["GATEWAY_SERVICE_TOKEN", "GATEWAY_URL"]);
    expect(error.message).not.toContain("not-a-url-value");
    expect(error.message).not.toContain("too-short-token-value");
  });

  it("accepts timeouts only as whole milliseconds between 100 and 20000", () => {
    const boundaryEnvironment = parseEnvironment(environmentSchema, {
      ...validEnvironment,
      DATABASE_TIMEOUT_MS: "100",
      GATEWAY_TIMEOUT_MS: "20000",
    });
    expect(boundaryEnvironment.DATABASE_TIMEOUT_MS).toBe(100);
    expect(boundaryEnvironment.GATEWAY_TIMEOUT_MS).toBe(20000);

    const outOfRangeError = captureValidationError({
      ...validEnvironment,
      DATABASE_TIMEOUT_MS: "99",
      GATEWAY_TIMEOUT_MS: "20001",
    });
    expect(outOfRangeError.invalidVariables).toEqual(["DATABASE_TIMEOUT_MS", "GATEWAY_TIMEOUT_MS"]);

    const fractionalError = captureValidationError({
      ...validEnvironment,
      DATABASE_TIMEOUT_MS: "250.5",
    });
    expect(fractionalError.invalidVariables).toEqual(["DATABASE_TIMEOUT_MS"]);
  });

  it("rejects a wildcard CORS origin", () => {
    const error = captureValidationError({ ...validEnvironment, CORS_ALLOWED_ORIGINS: "*" });

    expect(error.invalidVariables).toEqual(["CORS_ALLOWED_ORIGINS"]);
  });
});
