import { Logger } from "@nestjs/common";
import type { NextFunction, Request, Response } from "express";
import { getRequestPath } from "./request-id.middleware.js";

const requestLogger = new Logger("HttpRequest");

/** Logs one line per finished request: method, path, status and duration. No headers, bodies or query strings. */
export function requestLoggingMiddleware(
  request: Request,
  response: Response,
  next: NextFunction,
): void {
  const startedAt = performance.now();
  response.once("finish", () => {
    requestLogger.log("request completed", {
      method: request.method,
      path: getRequestPath(request),
      statusCode: response.statusCode,
      durationMs: Math.round(performance.now() - startedAt),
      requestId: request.requestId,
    });
  });
  next();
}
