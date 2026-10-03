import { STATUS_CODES } from "node:http";
import {
  type ArgumentsHost,
  Catch,
  type ExceptionFilter,
  HttpException,
  HttpStatus,
  Logger,
} from "@nestjs/common";
import type { ErrorResponse } from "@workspace/contracts";
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
  "bad_request",
  "unauthorized",
  "forbidden",
  "not_found",
  "method_not_allowed",
  "internal_error",
  "not_implemented",
  "resource_out_of_scope",
  "destination_not_allowed",
  "report_export_restricted",
  "report_lineage_missing",
  "source_policy_changed",
  "template_not_allowed",
  "approval_required",
  "approval_expired",
  "action_changed",
  "resource_version_changed",
  "allowance_exhausted",
  "run_cancelled",
  "outcome_unknown",
  "semantic_injection_detected",
  "security_evaluator_unavailable",
  "security_allowance_exhausted",
  "content_redacted",
  "signature_match",
  "policy_reload_rejected",
  "model_not_allowed",
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
    if (
      typeof payload === "object" &&
      payload !== null &&
      "code" in payload &&
      typeof (payload as any).code === "string"
    ) {
      const code = (payload as any).code;
      if (KNOWN_ERROR_CODES.has(code)) {
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
      // Full detail goes to the server log only.
      this.logger.error("request failed", {
        requestId: request.requestId,
        path: requestPath,
        statusCode,
        errorName: exception instanceof Error ? exception.name : typeof exception,
        errorMessage: exception instanceof Error ? exception.message : String(exception),
        stack: exception instanceof Error ? exception.stack : undefined,
      });
    }

    const body: ErrorResponse = {
      error: {
        code: resolveErrorCode(statusCode, exception),
        message: resolveSafeMessage(exception, statusCode),
      },
      statusCode,
      requestId: request.requestId,
      timestamp: new Date().toISOString(),
      path: requestPath,
    };
    response.status(statusCode).json(body);
  }
}
