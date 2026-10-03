import { NotImplementedException } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import { describe, expect, it } from "vitest";
import { AuthModule } from "./auth.module.js";
import { AUTH_PROVIDER, type AuthProvider } from "./auth.types.js";

describe("AuthModule placeholder", () => {
  it("binds a provider that rejects with 501 instead of returning an identity", async () => {
    const moduleRef = await Test.createTestingModule({ imports: [AuthModule] }).compile();
    const authProvider = moduleRef.get<AuthProvider>(AUTH_PROVIDER);

    await expect(authProvider.authenticate("any-credential")).rejects.toBeInstanceOf(
      NotImplementedException,
    );
  });
});
