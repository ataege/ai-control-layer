import { Injectable, Logger } from "@nestjs/common";
import {
  type DiagnosticCheck,
  type DiagnosticFailureReason,
  REQUEST_ID_HEADER,
  type OperatorContext,
} from "@workspace/contracts";
import { AppConfigService } from "../config/app-config.service.js";
import { z } from "zod";
import { SignJWT } from "jose";
import { randomUUID } from "node:crypto";

export type CommandFailureReason =
  | "timeout"
  | "unreachable"
  | "unauthorized"
  | "bad_request"
  | "server_error"
  | "invalid_response"
  | "unexpected_status"
  | "network_error";

export type CommandOutcome<T> =
  { success: true; data: T } | { success: false; reason: CommandFailureReason; code?: string };

const errorEnvelopeSchema = z.object({
  error: z.object({
    code: z.string(),
    message: z.string().optional(),
  }),
});

const PING_PATH = "/internal/ping";
const READINESS_PATH = "/health/ready";

/** Decides from a received response whether the check failed. `undefined` means it passed. */
type ResponseInterpreter = (response: Response) => Promise<DiagnosticFailureReason | undefined>;

function isTimeoutError(error: unknown): boolean {
  // AbortSignal.timeout rejects with a DOMException named TimeoutError.
  return error instanceof DOMException && error.name === "TimeoutError";
}

// Reads a small JSON body; malformed JSON yields undefined, a timeout is rethrown.
async function readJsonObject(response: Response): Promise<Record<string, unknown> | undefined> {
  try {
    const parsedBody: unknown = await response.json();
    return typeof parsedBody === "object" && parsedBody !== null
      ? (parsedBody as Record<string, unknown>)
      : undefined;
  } catch (error) {
    if (isTimeoutError(error)) {
      throw error;
    }
    return undefined;
  }
}

async function interpretPingResponse(
  response: Response,
): Promise<DiagnosticFailureReason | undefined> {
  if (response.status === 401 || response.status === 403) {
    return "unauthorized";
  }
  if (response.status !== 200) {
    return "unexpected_response";
  }
  const body = await readJsonObject(response);
  return body?.status === "ok" && body.service === "gateway" ? undefined : "unexpected_response";
}

async function interpretReadinessResponse(
  response: Response,
): Promise<DiagnosticFailureReason | undefined> {
  if (response.status === 503) {
    return "not_ready";
  }
  if (response.status !== 200) {
    return "unexpected_response";
  }
  const body = await readJsonObject(response);
  return body?.status === "ok" ? undefined : "unexpected_response";
}

/**
 * Outbound calls to the Go gateway. Every method returns a sanitized DiagnosticCheck:
 * the token, the gateway URL and upstream bodies are never returned or logged.
 */
@Injectable()
export class GatewayClientService {
  private readonly logger = new Logger(GatewayClientService.name);

  private readonly operatorKey: Uint8Array;

  constructor(private readonly config: AppConfigService) {
    this.operatorKey = new TextEncoder().encode(this.config.operatorContextSigningKey);
  }

  private async buildToken(context: OperatorContext): Promise<string> {
    const jwt = new SignJWT({ ctx: context })
      .setProtectedHeader({ alg: "HS256" })
      .setIssuedAt()
      .setAudience("gateway")
      .setExpirationTime("1m")
      .setJti(randomUUID())
      .setIssuer("gateway-client");

    return await jwt.sign(this.operatorKey);
  }

  /** Authenticated GET /internal/ping: proves service-to-service reachability. */
  async ping(requestId: string): Promise<DiagnosticCheck> {
    return this.probe(
      PING_PATH,
      requestId,
      { authorization: `Bearer ${this.config.gatewayServiceToken}` },
      interpretPingResponse,
    );
  }

  /** GET /health/ready: reports whether the gateway can reach its database. */
  checkReadiness(requestId: string): Promise<DiagnosticCheck> {
    return this.probe(READINESS_PATH, requestId, {}, interpretReadinessResponse);
  }

  private async probe(
    path: string,
    requestId: string,
    extraHeaders: Record<string, string>,
    interpretResponse: ResponseInterpreter,
  ): Promise<DiagnosticCheck> {
    const startedAt = performance.now();
    const measureLatencyMs = (): number => Math.round(performance.now() - startedAt);

    let check: DiagnosticCheck;
    try {
      const response = await fetch(new URL(path, this.config.gatewayUrl), {
        method: "GET",
        headers: { accept: "application/json", [REQUEST_ID_HEADER]: requestId, ...extraHeaders },
        // One deadline covers connecting, headers and the body.
        signal: AbortSignal.timeout(this.config.gatewayTimeoutMs),
        // Never follow redirects: the bearer token must not travel to another location.
        redirect: "manual",
      });
      const failureReason = await interpretResponse(response);
      // Release the socket when the body was not read.
      await response.body?.cancel().catch(() => undefined);
      check =
        failureReason === undefined
          ? { status: "up", latencyMs: measureLatencyMs(), upstreamStatus: response.status }
          : {
              status: "down",
              latencyMs: measureLatencyMs(),
              upstreamStatus: response.status,
              reason: failureReason,
            };
    } catch (error) {
      check = {
        status: "down",
        latencyMs: measureLatencyMs(),
        reason: isTimeoutError(error) ? "timeout" : "unreachable",
      };
    }

    if (check.status === "down") {
      this.logger.warn("gateway check failed", {
        path,
        reason: check.reason,
        upstreamStatus: check.upstreamStatus,
        requestId,
      });
    }
    return check;
  }

  /** Performs an authenticated POST request to the gateway, expecting a Zod-validated response. */
  async postCommand<Schema extends z.ZodTypeAny>(
    path: string,
    requestId: string,
    body: unknown,
    responseSchema: Schema,
    context: OperatorContext,
  ): Promise<CommandOutcome<z.infer<Schema>>> {
    try {
      const headers: Record<string, string> = {
        accept: "application/json",
        "content-type": "application/json",
        [REQUEST_ID_HEADER]: requestId,
        authorization: `Bearer ${this.config.gatewayServiceToken}`,
      };
      // Every command carries the verified operator context; there is no fallback identity.
      headers["x-operator-context"] = await this.buildToken(context);

      const response = await fetch(new URL(path, this.config.gatewayUrl), {
        method: "POST",
        headers,
        body: JSON.stringify(body),
        redirect: "manual",
        signal: AbortSignal.timeout(this.config.commandTimeoutMs),
      });

      if (response.status >= 200 && response.status < 300) {
        const json = await readJsonObject(response);
        if (!json) {
          return { success: false, reason: "invalid_response" };
        }
        const parsed = responseSchema.safeParse(json);
        if (parsed.success) {
          return { success: true, data: parsed.data };
        }
        return { success: false, reason: "invalid_response" };
      }

      if (response.status === 401 || response.status === 403) {
        return { success: false, reason: "unauthorized" };
      }

      if (response.status >= 400 && response.status < 500) {
        const json = await readJsonObject(response);
        const parsedError = errorEnvelopeSchema.safeParse(json);
        if (parsedError.success) {
          return { success: false, reason: "bad_request", code: parsedError.data.error.code };
        }
        return { success: false, reason: "bad_request" };
      }

      if (response.status >= 500) {
        const json = await readJsonObject(response);
        const parsedError = errorEnvelopeSchema.safeParse(json);
        if (parsedError.success) {
          return { success: false, reason: "server_error", code: parsedError.data.error.code };
        }
        return { success: false, reason: "server_error" };
      }

      return { success: false, reason: "unexpected_status" };
    } catch (error) {
      if (isTimeoutError(error)) {
        return { success: false, reason: "timeout" };
      }
      return { success: false, reason: "unreachable" };
    }
  }
}
