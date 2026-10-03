import { ApiExtraModels, ApiProperty, ApiPropertyOptional, getSchemaPath } from "@nestjs/swagger";
import type {
  ApiHealthIndicator,
  ApiReadinessResponse,
  LivenessResponse,
} from "@workspace/contracts";

// The classes implement the contract interfaces so drift is a compile error.

export class LivenessResponseDto implements LivenessResponse {
  @ApiProperty({ enum: ["ok"] })
  status: "ok";

  @ApiProperty({ enum: ["api"] })
  service: "api";
}

export class ApiHealthIndicatorDto implements ApiHealthIndicator {
  @ApiProperty({ enum: ["up", "down"] })
  status: "up" | "down";

  @ApiPropertyOptional({ description: "Short sanitized reason, present when the check is down." })
  message?: string;
}

const indicatorMapSchema = {
  type: "object",
  additionalProperties: { $ref: getSchemaPath(ApiHealthIndicatorDto) },
} as const;

@ApiExtraModels(ApiHealthIndicatorDto)
export class ApiReadinessResponseDto implements ApiReadinessResponse {
  @ApiProperty({ enum: ["ok", "error", "shutting_down"] })
  status: "ok" | "error" | "shutting_down";

  @ApiPropertyOptional({ ...indicatorMapSchema, description: "Indicators that are up." })
  info?: Record<string, ApiHealthIndicatorDto>;

  @ApiPropertyOptional({ ...indicatorMapSchema, description: "Indicators that are down." })
  error?: Record<string, ApiHealthIndicatorDto>;

  @ApiProperty({ ...indicatorMapSchema, description: "Every indicator." })
  details: Record<string, ApiHealthIndicatorDto>;
}
