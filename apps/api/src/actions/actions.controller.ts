import {
  Controller,
  Body,
  BadRequestException,
  ForbiddenException,
  Get,
  Header,
  HttpCode,
  Param,
  Post,
  Req,
  ServiceUnavailableException,
} from "@nestjs/common";
import { ApiOperation, ApiTags } from "@nestjs/swagger";
import type { ReviewView, ApprovalResponse, ApprovalDecision } from "@workspace/contracts";
import contract from "@workspace/contracts/schemas/review-view.schema.json" with { type: "json" };
import decisionContract from "@workspace/contracts/schemas/approval-decision.schema.json" with { type: "json" };
import responseContract from "@workspace/contracts/schemas/approval-response.schema.json" with { type: "json" };
import type { Request } from "express";
import { z } from "zod";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import {
  gatewayData,
  requireRecordId,
  verifiedOperator,
} from "../gateway-client/gateway-response.js";

const { allOf: conditions, ...shape } = contract;
const decisionSchema = z.fromJSONSchema(decisionContract as Parameters<typeof z.fromJSONSchema>[0]);
const responseSchema = z.fromJSONSchema(responseContract as Parameters<typeof z.fromJSONSchema>[0]);
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

  @Post(":id/approval")
  @HttpCode(200)
  @Header("Cache-Control", "no-store")
  @ApiOperation({
    summary: "Forward approve or reject for the exact frozen action; reviewer role required",
  })
  async approval(
    @Param("id") actionId: string,
    @Body() body: unknown,
    @Req() request: Request,
  ): Promise<ApprovalResponse> {
    const operator = verifiedOperator(request);
    if (!operator.roles.includes("reviewer"))
      throw new ForbiddenException("Reviewer role required");
    requireRecordId(actionId);
    const parsed = decisionSchema.safeParse(body);
    if (!parsed.success) throw new BadRequestException("Invalid approval decision");
    const decision = parsed.data as ApprovalDecision;
    const result = gatewayData(
      await this.gateway.postCommand(
        `/internal/actions/${actionId}/approval`,
        request.requestId,
        decision,
        responseSchema,
        operator,
      ),
      true,
    ) as ApprovalResponse;
    if (result.actionId !== actionId || result.decision !== decision.decision) {
      throw new ServiceUnavailableException("Invalid gateway approval reference");
    }
    return result;
  }

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
