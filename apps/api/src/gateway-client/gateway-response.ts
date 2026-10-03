import { BadRequestException, HttpException, InternalServerErrorException } from "@nestjs/common";
import type { OperatorContext } from "@workspace/contracts";
import type { Request } from "express";
import type { CommandOutcome } from "./gateway-client.service.js";

export function verifiedOperator(request: Request): OperatorContext {
  if (!request.operatorContext) {
    throw new InternalServerErrorException("Missing operator context");
  }
  return request.operatorContext;
}

export function requireRecordId(id: string): void {
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(id)) {
    throw new BadRequestException("Invalid record identifier");
  }
}

/** Keep received error statuses; a missing or invalid answer never becomes success. */
export function gatewayData<T>(outcome: CommandOutcome<T>, command = false): T {
  if (outcome.success) return outcome.data;
  if (command && outcome.reason === "timeout") {
    throw new HttpException(
      {
        code: "outcome_unconfirmed",
        message: "The request timed out; the outcome is unconfirmed.",
      },
      504,
    );
  }
  const statusCode =
    outcome.statusCode !== undefined && outcome.statusCode >= 400
      ? outcome.statusCode
      : outcome.reason === "unauthorized"
        ? 401
        : 503;
  throw new HttpException(
    { code: outcome.code ?? "upstream_unavailable", message: "Gateway request failed" },
    statusCode,
  );
}
