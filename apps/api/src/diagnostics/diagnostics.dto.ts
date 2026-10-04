import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type {
  DiagnosticCheck,
  DiagnosticFailureReason,
  GatewayDiagnosticsResponse,
} from "@workspace/contracts";

// The classes implement the contract interfaces so drift is a compile error.

const FAILURE_REASONS: DiagnosticFailureReason[] = [
  "timeout",
  "unreachable",
  "unauthorized",
  "not_ready",
  "unexpected_response",
];

export class DiagnosticCheckDto implements DiagnosticCheck {
  @ApiProperty({ enum: ["up", "down"] })
  status: "up" | "down";

  @ApiProperty({ description: "Round-trip time measured by the API in milliseconds.", minimum: 0 })
  latencyMs: number;

  @ApiPropertyOptional({
    type: "integer",
    minimum: 100,
    maximum: 599,
    description: "HTTP status returned by the gateway, when a response was received.",
  })
  upstreamStatus?: number;

  @ApiPropertyOptional({ enum: FAILURE_REASONS, description: "Present only when status is down." })
  reason?: DiagnosticFailureReason;
}

export class GatewayDiagnosticsChecksDto {
  @ApiProperty({ type: DiagnosticCheckDto, description: "Authenticated GET /internal/ping." })
  reachability: DiagnosticCheckDto;

  @ApiProperty({
    type: DiagnosticCheckDto,
    description:
      "GET /health/ready on the gateway. The existing field includes worker/catalog readiness; 503 remains down even if the database is up.",
  })
  databaseReadiness: DiagnosticCheckDto;
}

export class GatewayDiagnosticsResponseDto implements GatewayDiagnosticsResponse {
  @ApiProperty({ enum: ["ok", "degraded", "unavailable"] })
  status: "ok" | "degraded" | "unavailable";

  @ApiProperty({ minLength: 1 })
  requestId: string;

  @ApiProperty({ type: GatewayDiagnosticsChecksDto })
  checks: GatewayDiagnosticsChecksDto;
}
