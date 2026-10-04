import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type { StartRunRequest, StartRunResponse } from "@workspace/contracts";
import { z } from "zod";

export const StartRunSchema = z
  .object({
    template: z.string().min(1).max(256),
    vendorId: z.string().max(256).optional(),
    invoiceIds: z.array(z.string().max(256)).max(100),
    destination: z.string().min(1).max(256),
    approvalRequirement: z.string().max(256).optional(),
    limits: z
      .object({
        modelCalls: z.number().int().positive().max(1000).optional(),
        timeoutSeconds: z.number().int().positive().max(3600).optional(),
      })
      .strict()
      .optional(),
  })
  .strict();

// Swagger documentation of X-07 (API-11). The classes implement the contract types, so drift is a
// compile error; StartRunSchema above stays the validator.

export class StartRunLimitsDto implements NonNullable<StartRunRequest["limits"]> {
  @ApiPropertyOptional({
    type: "integer",
    minimum: 1,
    maximum: 1000,
    description: "Requested model calls; admission refuses more than the active catalog allows.",
  })
  modelCalls?: number;

  @ApiPropertyOptional({
    type: "integer",
    minimum: 1,
    maximum: 3600,
    description:
      "Requested run lifetime in seconds; admission refuses more than the catalog allows.",
  })
  timeoutSeconds?: number;
}

export class StartRunRequestDto implements StartRunRequest {
  @ApiProperty({ minLength: 1, maxLength: 256, example: "reconcile_atlas_v1" })
  template: string;

  @ApiPropertyOptional({ maxLength: 256, example: "vendor_Atlas" })
  vendorId?: string;

  @ApiProperty({
    type: [String],
    maxItems: 100,
    example: ["invoice_A01", "invoice_A02"],
    description: "Invoices of one vendor of the operator's organization.",
  })
  invoiceIds: string[];

  @ApiProperty({ minLength: 1, maxLength: 256, example: "vendor_Atlas" })
  destination: string;

  @ApiPropertyOptional({ maxLength: 256, example: "review_queue_report" })
  approvalRequirement?: string;

  @ApiPropertyOptional({ type: StartRunLimitsDto })
  limits?: StartRunLimitsDto;
}

export class StartRunResponseDto implements StartRunResponse {
  @ApiProperty({ format: "uuid" })
  runId: string;

  @ApiProperty({ format: "uuid" })
  passportId: string;
}
