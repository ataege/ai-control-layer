import {
  Controller,
  ForbiddenException,
  Get,
  Header,
  Param,
  Req,
  ServiceUnavailableException,
} from "@nestjs/common";
import { ApiOperation, ApiTags } from "@nestjs/swagger";
import type { ReviewView } from "@workspace/contracts";
import contract from "@workspace/contracts/schemas/review-view.schema.json" with { type: "json" };
import type { Request } from "express";
import { z } from "zod";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import {
  gatewayData,
  requireRecordId,
  verifiedOperator,
} from "../gateway-client/gateway-response.js";

const { allOf: conditions, ...shape } = contract;
const queueArgumentsSchema = z.fromJSONSchema(
  conditions[0]!.then.properties.canonical_arguments as Parameters<typeof z.fromJSONSchema>[0],
);
const reviewSchema = z
  .fromJSONSchema(shape as Parameters<typeof z.fromJSONSchema>[0])
  .superRefine((value, context) => {
    const payload = value as ReviewView;
    if (
      payload.tool === "queue_report"
        ? payload.recipient === null ||
          payload.report === null ||
          !queueArgumentsSchema.safeParse(payload.canonical_arguments).success
        : payload.recipient !== null || payload.report !== null
    ) {
      context.addIssue({ code: "custom", message: "Invalid frozen review material" });
    }
  });

@ApiTags("actions")
@Controller("actions")
export class ActionsController {
  constructor(private readonly gateway: GatewayClientService) {}

  @Get(":id/review")
  @Header("Cache-Control", "no-store")
  @ApiOperation({ summary: "Read the exact frozen review payload; reviewer role required" })
  async review(@Param("id") actionId: string, @Req() request: Request): Promise<ReviewView> {
    const operator = verifiedOperator(request);
    if (!operator.roles.includes("reviewer"))
      throw new ForbiddenException("Reviewer role required");
    requireRecordId(actionId);
    const payload = gatewayData(
      await this.gateway.getRead(
        `/internal/actions/${actionId}/review`,
        request.requestId,
        reviewSchema,
        operator,
      ),
    ) as ReviewView;
    if (payload.action_id !== actionId)
      throw new ServiceUnavailableException("Invalid gateway action reference");
    return payload;
  }
}
