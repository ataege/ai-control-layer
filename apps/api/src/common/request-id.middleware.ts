import { randomUUID } from "node:crypto";
import { REQUEST_ID_HEADER } from "@workspace/contracts";
import type { NextFunction, Request, Response } from "express";

// Same rule as the gateway: anything else is replaced, so untrusted input never reaches logs.
const ACCEPTED_REQUEST_ID = /^[A-Za-z0-9._-]{1,64}$/;

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Express {
    interface Request {
      /** Correlation id set by requestIdMiddleware. */
      requestId: string;
    }
  }
}

/** Reuses a well-formed inbound x-request-id or generates one, and echoes it on the response. */
export function requestIdMiddleware(
  request: Request,
  response: Response,
  next: NextFunction,
): void {
  const inboundRequestId = request.header(REQUEST_ID_HEADER);
  const requestId =
    inboundRequestId !== undefined && ACCEPTED_REQUEST_ID.test(inboundRequestId)
      ? inboundRequestId
      : randomUUID();
  request.requestId = requestId;
  response.setHeader(REQUEST_ID_HEADER, requestId);
  next();
}

/** Path of the original request URL without the query string (safe to log and return). */
export function getRequestPath(request: Request): string {
  return request.originalUrl.split("?", 1)[0] ?? "/";
}
