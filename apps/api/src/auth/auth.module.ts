import { Module } from "@nestjs/common";
import { APP_GUARD } from "@nestjs/core";
import { AUTH_PROVIDER } from "./auth.types.js";
import { DefaultDenyGuard } from "./default-deny.guard.js";
import { CookieAuthProvider } from "./cookie-auth.provider.js";
import { AuthController } from "./auth.controller.js";
import { IdentityModule } from "../identity/identity.module.js";

/**
 * Stored-session authentication and the global current-membership guard.
 */
@Module({
  imports: [IdentityModule],
  controllers: [AuthController],
  providers: [
    { provide: APP_GUARD, useClass: DefaultDenyGuard },
    { provide: AUTH_PROVIDER, useClass: CookieAuthProvider },
  ],
  exports: [AUTH_PROVIDER],
})
export class AuthModule {}
