import { Module } from "@nestjs/common";
import { AUTH_PROVIDER } from "./auth.types.js";
import { UnimplementedAuthProvider } from "./unimplemented-auth.provider.js";

/**
 * Extension point: replace the AUTH_PROVIDER binding with a real provider and add a guard
 * when authentication is introduced. Nothing here is applied to any route.
 */
@Module({
  providers: [{ provide: AUTH_PROVIDER, useClass: UnimplementedAuthProvider }],
  exports: [AUTH_PROVIDER],
})
export class AuthModule {}
