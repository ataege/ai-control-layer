import { NestFactory } from "@nestjs/core";
import type { NestExpressApplication } from "@nestjs/platform-express";
import { AppModule } from "./app.module.js";
import { configureApp } from "./app.setup.js";
import { createApplicationLogger } from "./common/logger.js";
import { AppConfigService } from "./config/app-config.service.js";
import {
  EnvironmentValidationError,
  environmentSchema,
  parseEnvironment,
} from "./config/environment.js";
import { setupOpenApi } from "./openapi.js";

const REQUEST_TIMEOUT_MS = 30_000;
const HEADERS_TIMEOUT_MS = 15_000;
const KEEP_ALIVE_TIMEOUT_MS = 5_000;

// Importing AppModule has already loaded the root .env, so LOG_LEVEL from the file is visible here.
const logger = createApplicationLogger(process.env.LOG_LEVEL);

async function bootstrap(): Promise<void> {
  // Fail fast with one clear message before Nest builds anything.
  parseEnvironment(environmentSchema, process.env);

  const app = await NestFactory.create<NestExpressApplication>(AppModule, {
    logger,
    // Startup failures are reported once by the handler below instead of Nest's default abort.
    abortOnError: false,
  });
  const config = app.get(AppConfigService);

  configureApp(app, config.corsAllowedOrigins, config.cookieSecret);
  setupOpenApi(app);
  // SIGTERM/SIGINT: stop accepting connections, drain in-flight requests, close the DataSource, exit 0.
  app.enableShutdownHooks(["SIGTERM", "SIGINT"], { useProcessExit: true });

  // Bound slow or idle clients. Set before listening so the first connection is covered.
  const httpServer = app.getHttpServer();
  httpServer.requestTimeout = REQUEST_TIMEOUT_MS;
  httpServer.headersTimeout = HEADERS_TIMEOUT_MS;
  httpServer.keepAliveTimeout = KEEP_ALIVE_TIMEOUT_MS;

  await app.listen(config.apiPort, config.apiHost);
  logger.log(`API listening on ${config.apiHost}:${config.apiPort}`, "Bootstrap");
}

try {
  await bootstrap();
} catch (error) {
  // Configuration errors name variables only; other errors keep their stack in the server log.
  if (error instanceof EnvironmentValidationError) {
    logger.error(error.message, "Bootstrap");
  } else {
    logger.error(
      "API failed to start",
      error instanceof Error ? error.stack : undefined,
      "Bootstrap",
    );
  }
  process.exit(1);
}
