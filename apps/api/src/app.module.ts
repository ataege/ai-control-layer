import { Module } from "@nestjs/common";
import { AuthModule } from "./auth/auth.module.js";
import { AppConfigModule } from "./config/app-config.module.js";
import { DatabaseModule } from "./database/database.module.js";
import { DiagnosticsModule } from "./diagnostics/diagnostics.module.js";
import { HealthModule } from "./health/health.module.js";
import { RegistryModule } from "./registry/registry.module.js";
import { IdentityModule } from "./identity/identity.module.js";
import { RunsModule } from "./runs/runs.module.js";
import { SecurityModule } from "./security/security.module.js";
import { ActionsModule } from "./actions/actions.module.js";
import { PoliciesModule } from "./policies/policies.module.js";

@Module({
  imports: [
    AppConfigModule,
    DatabaseModule,
    HealthModule,
    DiagnosticsModule,
    AuthModule,
    RegistryModule,
    IdentityModule,
    RunsModule,
    SecurityModule,
    ActionsModule,
    PoliciesModule,
  ],
})
export class AppModule {}
