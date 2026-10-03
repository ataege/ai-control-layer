import { Module } from "@nestjs/common";
import { TypeOrmModule } from "@nestjs/typeorm";
import { AppConfigService } from "../config/app-config.service.js";
import { DatabaseInitializerService } from "./database-initializer.service.js";
import { buildTypeOrmOptions } from "./typeorm-options.js";

@Module({
  imports: [
    TypeOrmModule.forRootAsync({
      inject: [AppConfigService],
      useFactory: (config: AppConfigService) => ({
        ...buildTypeOrmOptions(config.database),
        // Do not connect during module init; DatabaseInitializerService connects in the background.
        manualInitialization: true,
      }),
    }),
  ],
  providers: [DatabaseInitializerService],
})
export class DatabaseModule {}
