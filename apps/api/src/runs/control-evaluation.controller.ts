import {
  BadRequestException,
  Body,
  Controller,
  Header,
  HttpCode,
  Post,
  Req,
  ServiceUnavailableException,
} from "@nestjs/common";
import { ApiOperation, ApiTags } from "@nestjs/swagger";
import type { ControlEvaluationRequest, ControlEvaluationResponse } from "@workspace/contracts";
import requestContract from "@workspace/contracts/schemas/control-evaluation-request.schema.json" with { type: "json" };
import responseContract from "@workspace/contracts/schemas/control-evaluation-response.schema.json" with { type: "json" };
import type { Request } from "express";
import { z } from "zod";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { gatewayData, verifiedOperator } from "../gateway-client/gateway-response.js";

const { allOf: conditions, ...requestShape } = requestContract;
const inputSchema = z
  .fromJSONSchema(requestShape as Parameters<typeof z.fromJSONSchema>[0])
  .superRefine((value, context) => {
    const input = value as ControlEvaluationRequest;
    const condition = conditions.find(
      (condition) => condition.if.properties.kind.const === input.kind,
    );
    if (
      !condition ||
      !z
        .fromJSONSchema({ type: "object", properties: condition.then.properties } as Parameters<
          typeof z.fromJSONSchema
        >[0])
        .safeParse(input).success
    ) {
      context.addIssue({ code: "custom", message: "Invalid evaluation boundary input" });
    }
  });
const responseSchema = z.fromJSONSchema(responseContract as Parameters<typeof z.fromJSONSchema>[0]);

@ApiTags("control")
@Controller("control")
export class ControlEvaluationController {
  constructor(private readonly gateway: GatewayClientService) {}

  @Post("evaluate")
  @HttpCode(200)
  @Header("Cache-Control", "no-store")
  @ApiOperation({ summary: "Evaluate judge input on an admitted run through Go's security gates" })
  async evaluate(
    @Body() body: unknown,
    @Req() request: Request,
  ): Promise<ControlEvaluationResponse> {
    const operator = verifiedOperator(request);
    const parsed = inputSchema.safeParse(body);
    if (!parsed.success) throw new BadRequestException("Invalid control evaluation request");
    const input = parsed.data as ControlEvaluationRequest;
    const result = gatewayData(
      await this.gateway.postCommand(
        "/internal/control/evaluate",
        request.requestId,
        input,
        responseSchema,
        operator,
      ),
      true,
    ) as ControlEvaluationResponse;
    if (result.runId !== input.runId)
      throw new ServiceUnavailableException("Invalid gateway evaluation reference");
    return result;
  }
}
