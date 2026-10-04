import { STATUS_CODES } from "node:http";
import {
  type ArgumentsHost,
  Catch,
  type ExceptionFilter,
  HttpException,
  HttpStatus,
  Logger,
} from "@nestjs/common";
import type { PolicyReloadErrorResponse } from "@workspace/contracts";
import reasonContract from "@workspace/contracts/schemas/reason-code.schema.json" with { type: "json" };
import type { Request, Response } from "express";
import { getRequestPath } from "./request-id.middleware.js";

// Stable codes shared with the gateway; other statuses derive a code from the status text.
const ERROR_CODES_BY_STATUS: Record<number, string> = {
  400: "bad_request",
  401: "unauthorized",
  403: "forbidden",
  404: "not_found",
  405: "method_not_allowed",
  500: "internal_error",
  501: "not_implemented",
};

const KNOWN_ERROR_CODES = new Set([
  ...reasonContract.enum,
  "conflict",
  "revision_pending",
  "bad_request",
  "unauthorized",
  "forbidden",
  "not_found",
  "method_not_allowed",
  "internal_error",
  "not_implemented",
  "outcome_unconfirmed",
  "upstream_unavailable",
  "upstream_unreachable",
  "upstream_timeout",
  "invalid_json",
  "configuration_error",
  "timeout",
  "network_error",
  "server_error",
  "unexpected_status",
]);

function resolveErrorCode(statusCode: number, exception: unknown): string {
  if (exception instanceof HttpException) {
    const payload = exception.getResponse();
    if (typeof payload === "object" && payload !== null && "code" in payload) {
      const code: unknown = payload.code;
      if (typeof code === "string" && KNOWN_ERROR_CODES.has(code)) {
        return code;
      }
    }
  }

  const knownCode = ERROR_CODES_BY_STATUS[statusCode];
  if (knownCode !== undefined) {
    return knownCode;
  }
  const statusText = STATUS_CODES[statusCode];
  return statusText === undefined ? "error" : statusText.toLowerCase().replace(/[^a-z0-9]+/g, "_");
}

function resolveSafeMessage(exception: unknown, statusCode: number): string {
  // 5xx details stay in the server log. 404 uses a fixed text because Nest's message echoes the URL.
  if (statusCode >= 500 || statusCode === 404 || !(exception instanceof HttpException)) {
    return statusCode === 404 ? "Resource not found" : (STATUS_CODES[statusCode] ?? "Error");
  }
  const payload = exception.getResponse();
  if (typeof payload === "string") {
    return payload;
  }
  const payloadMessage = (payload as { message?: unknown }).message;
  if (Array.isArray(payloadMessage)) {
    return payloadMessage.map(String).join("; ");
  }
  return typeof payloadMessage === "string" ? payloadMessage : exception.message;
}

// Body-parser style errors (e.g. 413 payload too large) carry their own client status.
function resolveStatusCode(exception: unknown): number {
  if (exception instanceof HttpException) {
    return exception.getStatus();
  }
  if (typeof exception === "object" && exception !== null) {
    const { statusCode, status } = exception as { statusCode?: unknown; status?: unknown };
    for (const candidateStatus of [statusCode, status]) {
      // Only 4xx is honoured: a foreign 5xx or out-of-range value stays a plain 500.
      if (
        Number.isInteger(candidateStatus) &&
        Number(candidateStatus) >= 400 &&
        Number(candidateStatus) <= 499
      ) {
        return Number(candidateStatus);
      }
    }
  }
  return HttpStatus.INTERNAL_SERVER_ERROR;
}

/** Turns every unhandled error into the shared ErrorResponse envelope. */
@Catch()
export class AllExceptionsFilter implements ExceptionFilter {
  private readonly logger = new Logger(AllExceptionsFilter.name);

  catch(exception: unknown, host: ArgumentsHost): void {
    const httpContext = host.switchToHttp();
    const request = httpContext.getRequest<Request>();
    const response = httpContext.getResponse<Response>();

    const statusCode = resolveStatusCode(exception);

    const requestPath = getRequestPath(request);

    if (statusCode >= 500) {
      // General logs carry correlation metadata only; exception text may contain credentials
      // or protected review content from an unavailable dependency.
      this.logger.error("request failed", {
        requestId: request.requestId,
        path: requestPath,
        statusCode,
        errorName:
          exception instanceof HttpException
            ? "HttpException"
            : exception instanceof Error
              ? "Error"
              : typeof exception,
      });
    }

    const body: PolicyReloadErrorResponse = {
      error: {
        code: resolveErrorCode(statusCode, exception),
        message: resolveSafeMessage(exception, statusCode),
      },
      statusCode,
      requestId: request.requestId,
      timestamp: new Date().toISOString(),
      path: requestPath,
    };
    // Only this NestJS-owned validation envelope includes the importer's safe issue list.
    if (
      statusCode === 400 &&
      body.error.code === "policy_reload_rejected" &&
      exception instanceof HttpException
    ) {
      const payload = exception.getResponse() as { issues?: unknown };
      if (
        Array.isArray(payload.issues) &&
        payload.issues.every(
          (issue: unknown) =>
            typeof issue === "object" &&
            issue !== null &&
            "path" in issue &&
            typeof issue.path === "string" &&
            "message" in issue &&
            typeof issue.message === "string",
        )
      ) {
        body.error.issues = payload.issues.map((issue: { path: string; message: string }) => ({
          path: issue.path,
          message: issue.message,
        }));
      }
    }
    response.status(statusCode).json(body);
  }
}
