import { Controller, Get, Header, Res, ServiceUnavailableException } from "@nestjs/common";
import {
  ApiOkResponse,
  ApiOperation,
  ApiServiceUnavailableResponse,
  ApiTags,
} from "@nestjs/swagger";
import { type HealthCheckResult, HealthCheckService } from "@nestjs/terminus";
import type { ApiHealthIndicator, ApiReadinessResponse } from "@workspace/contracts";
import type { Response } from "express";
import { DatabaseHealthIndicator } from "./database.health-indicator.js";
import { ApiReadinessResponseDto, LivenessResponseDto } from "./health.dto.js";
import { Public } from "../auth/public.decorator.js";

type IndicatorMap = HealthCheckResult["info"];

// Keeps only {status, message?} per indicator so nothing else can leak into the report.
function sanitizeIndicators(
  indicators: IndicatorMap | undefined,
): Record<string, ApiHealthIndicator> {
  const sanitizedIndicators: Record<string, ApiHealthIndicator> = {};
  for (const [indicatorKey, indicator] of Object.entries(indicators ?? {})) {
    if (indicator === undefined) {
      continue;
    }
    const message: unknown = indicator.message;
    sanitizedIndicators[indicatorKey] = {
      status: indicator.status === "up" ? "up" : "down",
      ...(typeof message === "string" ? { message } : {}),
    };
  }
  return sanitizedIndicators;
}

function toReadinessResponse(report: HealthCheckResult): ApiReadinessResponse {
  return {
    status: report.status === "ok" || report.status === "shutting_down" ? report.status : "error",
    info: sanitizeIndicators(report.info),
    error: sanitizeIndicators(report.error),
    details: sanitizeIndicators(report.details),
  };
}

@ApiTags("health")
@Public()
@Controller("health")
export class HealthController {
  constructor(
    private readonly healthCheckService: HealthCheckService,
    private readonly databaseHealthIndicator: DatabaseHealthIndicator,
  ) {}

  // Plain response on purpose: liveness must not depend on PostgreSQL, the gateway or terminus state.
  @Get("live")
  @Header("Cache-Control", "no-store")
  @ApiOperation({ summary: "Process liveness. No dependencies are checked." })
  @ApiOkResponse({ type: LivenessResponseDto })
  getLiveness(): LivenessResponseDto {
    return { status: "ok", service: "api" };
  }

  @Get("ready")
  @Header("Cache-Control", "no-store")
  @ApiOperation({ summary: "Readiness. Runs a read-only SELECT 1 against PostgreSQL." })
  @ApiOkResponse({ type: ApiReadinessResponseDto })
  @ApiServiceUnavailableResponse({ type: ApiReadinessResponseDto })
  async getReadiness(
    @Res({ passthrough: true }) response: Response,
  ): Promise<ApiReadinessResponseDto> {
    try {
      const report = await this.healthCheckService.check([
        () => this.databaseHealthIndicator.check("database"),
      ]);
      return toReadinessResponse(report);
    } catch (error) {
      // Terminus signals a failed check by throwing; the 503 body stays the report, not the error envelope.
      if (error instanceof ServiceUnavailableException) {
        response.status(503);
        return toReadinessResponse(error.getResponse() as HealthCheckResult);
      }
      throw error;
    }
  }
}
