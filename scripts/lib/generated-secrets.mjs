// The local secrets that `pnpm run setup` generates, shared with the smoke leak checks so that
// every generated secret is searched for automatically. Values are URL-safe base64 (4 chars per 3 bytes).
import { randomBytes } from "node:crypto";

export const GENERATED_SECRETS = {
  POSTGRES_PASSWORD: {
    label: "database password",
    generate: () => randomBytes(24).toString("base64url"), // 32 characters
  },
  POSTGRES_GATEWAY_PASSWORD: {
    label: "gateway database role password",
    generate: () => randomBytes(24).toString("base64url"), // 32 characters
  },
  GATEWAY_SERVICE_TOKEN: {
    label: "service token",
    generate: () => randomBytes(36).toString("base64url"), // 48 characters
  },
  AUTH_JWT_SECRET: {
    label: "session signing secret",
    generate: () => randomBytes(48).toString("base64url"), // 64 characters, 384 bits
  },
  OPERATOR_CONTEXT_SIGNING_KEY: {
    label: "operator-context signing key",
    generate: () => randomBytes(48).toString("base64url"), // 64 characters, 384 bits
  },
};
