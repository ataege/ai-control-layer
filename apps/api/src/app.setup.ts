import type { NestExpressApplication } from "@nestjs/platform-express";
import { REQUEST_ID_HEADER } from "@workspace/contracts";
import { AllExceptionsFilter } from "./common/all-exceptions.filter.js";
import { requestIdMiddleware } from "./common/request-id.middleware.js";
import { requestLoggingMiddleware } from "./common/request-logging.middleware.js";
import cookieParser from "cookie-parser";

export const GLOBAL_PREFIX = "api";

/** HTTP wiring shared by main.ts and the tests, so both exercise the same pipeline. */
export function configureApp(app: NestExpressApplication, corsAllowedOrigins: string[]): void {
  app.disable("x-powered-by");
  app.setGlobalPrefix(GLOBAL_PREFIX);
  // Request id first, so the access log and every error carry it (also for unknown routes).
  app.use(requestIdMiddleware, requestLoggingMiddleware, cookieParser());
  app.useGlobalFilters(new AllExceptionsFilter());
  // Explicit origins only; other origins receive no CORS headers.
  app.enableCors({
    origin: corsAllowedOrigins,
    methods: ["GET", "HEAD", "OPTIONS"],
    exposedHeaders: [REQUEST_ID_HEADER],
  });
}
