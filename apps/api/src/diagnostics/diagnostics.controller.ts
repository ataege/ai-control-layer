import { Controller, Get, Header, HttpStatus, Req, Res } from "@nestjs/common";
import {
  ApiBadGatewayResponse,
  ApiGatewayTimeoutResponse,
  ApiOkResponse,
  ApiOperation,
  ApiServiceUnavailableResponse,
  ApiTags,
} from "@nestjs/swagger";
import type { DiagnosticCheck, GatewayDiagnosticsResponse } from "@workspace/contracts";
import type { Request, Response } from "express";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { GatewayDiagnosticsResponseDto } from "./diagnostics.dto.js";

interface DiagnosticsOutcome {
  status: GatewayDiagnosticsResponse["status"];
  httpStatus: HttpStatus;
}

/** Maps the two checks to the overall status and the HTTP status of the diagnostics response. */
export function resolveDiagnosticsOutcome(
  reachability: DiagnosticCheck,
  databaseReadiness: DiagnosticCheck,
): DiagnosticsOutcome {
  if (reachability.status === "down") {
    return {
      status: "unavailable",
      httpStatus:
        reachability.reason === "timeout" ? HttpStatus.GATEWAY_TIMEOUT : HttpStatus.BAD_GATEWAY,
    };
  }
  if (databaseReadiness.status === "down") {
    return { status: "degraded", httpStatus: HttpStatus.SERVICE_UNAVAILABLE };
  }
  return { status: "ok", httpStatus: HttpStatus.OK };
}

@ApiTags("diagnostics")
@Controller("diagnostics")
export class DiagnosticsController {
  constructor(private readonly gatewayClient: GatewayClientService) {}

  @Get("gateway")
  @Header("Cache-Control", "no-store")
  @ApiOperation({
    summary: "Checks the gateway: authenticated ping and its database readiness.",
  })
  @ApiOkResponse({ type: GatewayDiagnosticsResponseDto, description: "Both checks passed." })
  @ApiServiceUnavailableResponse({
    type: GatewayDiagnosticsResponseDto,
    description: "Gateway reachable and authenticated, but not ready.",
  })
  @ApiBadGatewayResponse({
    type: GatewayDiagnosticsResponseDto,
    description: "Ping failed: unreachable, unauthorized or unexpected response.",
  })
  @ApiGatewayTimeoutResponse({
    type: GatewayDiagnosticsResponseDto,
    description: "Ping timed out.",
  })
  async getGatewayDiagnostics(
    @Req() request: Request,
    @Res({ passthrough: true }) response: Response,
  ): Promise<GatewayDiagnosticsResponseDto> {
    const requestId = request.requestId;
    // Both calls are bounded by GATEWAY_TIMEOUT_MS and never reject.
    const [reachability, databaseReadiness] = await Promise.all([
      this.gatewayClient.ping(requestId),
      this.gatewayClient.checkReadiness(requestId),
    ]);

    const outcome = resolveDiagnosticsOutcome(reachability, databaseReadiness);
    // The body is the diagnostics result for every status, not the error envelope.
    response.status(outcome.httpStatus);
    return { status: outcome.status, requestId, checks: { reachability, databaseReadiness } };
  }
}
