import { resolve } from "node:path";
import { Global, Module } from "@nestjs/common";
import { ConfigModule } from "@nestjs/config";
import { AppConfigService } from "./app-config.service.js";
import { environmentSchema, parseEnvironment } from "./environment.js";

// Monorepo-root .env, resolved from this file (src/config or dist/config), not from cwd.
const rootEnvFilePath = resolve(import.meta.dirname, "../../../../.env");

@Global()
@Module({
  imports: [
    // Only loads the file into process.env; real environment variables win. A missing file is fine.
    ConfigModule.forRoot({ envFilePath: rootEnvFilePath }),
  ],
  providers: [
    {
      provide: AppConfigService,
      // Validation runs when Nest instantiates providers (bootstrap), never at import or build time.
      useFactory: () => new AppConfigService(parseEnvironment(environmentSchema, process.env)),
    },
  ],
  exports: [AppConfigService],
})
export class AppConfigModule {}
