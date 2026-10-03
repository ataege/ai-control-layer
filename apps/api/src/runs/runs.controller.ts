import {
  Body,
  BadRequestException,
  Controller,
  Get,
  HttpException,
  InternalServerErrorException,
  Post,
  Param,
  Query,
  Req,
  ServiceUnavailableException,
  UsePipes,
} from "@nestjs/common";
import { ApiBody, ApiOperation, ApiResponse, ApiTags } from "@nestjs/swagger";
import type {
  OperatorContext,
  StartRunRequest,
  StartRunResponse,
  RunEventsPage,
} from "@workspace/contracts";
import type { Request } from "express";
import { z } from "zod";
import { ZodValidationPipe } from "../common/zod-validation.pipe.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { StartRunSchema } from "./dto/start-run.dto.js";
import { RunEventsSchema } from "./run-events.schema.js";

const StartRunResponseSchema = z.object({
  runId: z.string(),
  passportId: z.string(),
});

@ApiTags("runs")
@Controller("runs")
export class RunsController {
  constructor(private readonly gateway: GatewayClientService) {}

  @Get(":id/events")
  @ApiOperation({ summary: "Read an authorized page of sanitized run events" })
  async events(
    @Param("id") runId: string,
    @Query() query: Record<string, unknown>,
    @Req() request: Request,
  ): Promise<RunEventsPage> {
    if (!request.operatorContext) {
      throw new InternalServerErrorException("Missing operator context");
    }
    if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(runId)) {
      throw new BadRequestException("Invalid run identifier");
    }
    const parsed = z
      .strictObject({
        after: z
          .string()
          .regex(/^(0|[1-9][0-9]{0,18})$/)
          .refine((cursor) => BigInt(cursor) <= 9223372036854775807n)
          .optional(),
        limit: z
          .string()
          .regex(/^[1-9][0-9]{0,2}$/)
          .refine((limit) => Number(limit) <= 500)
          .optional(),
      })
      .safeParse(query);
    if (!parsed.success) {
      throw new BadRequestException("Invalid event pagination");
    }
    const parameters = new URLSearchParams(parsed.data);
    const outcome = await this.gateway.getRead(
      `/internal/runs/${runId}/events${parameters.size ? `?${parameters.toString()}` : ""}`,
      request.requestId,
      RunEventsSchema,
      request.operatorContext,
    );
    if (!outcome.success) {
      const statusCode =
        outcome.statusCode !== undefined && outcome.statusCode >= 400
          ? outcome.statusCode
          : outcome.reason === "unauthorized"
            ? 401
            : 503;
      throw new HttpException(
        { code: outcome.code ?? "upstream_unavailable", message: "Event read failed" },
        statusCode,
      );
    }
    const page = outcome.data as RunEventsPage;
    let previous = BigInt(parsed.data.after ?? "0");
    for (const event of page.events) {
      if (
        event.organizationId !== request.operatorContext.organizationId ||
        event.runId !== runId ||
        BigInt(event.eventId) <= previous
      ) {
        throw new ServiceUnavailableException("Invalid gateway event page");
      }
      previous = BigInt(event.eventId);
    }
    if (
      BigInt(page.nextCursor) !== previous ||
      page.events.length > Number(parsed.data.limit ?? 500)
    ) {
      throw new ServiceUnavailableException("Invalid gateway event page");
    }
    return page;
  }

  @Post()
  @ApiOperation({ summary: "Start a new run" })
  @ApiBody({
    schema: {
      type: "object",
      properties: {
        template: { type: "string" },
        vendorId: { type: "string" },
        invoiceIds: { type: "array", items: { type: "string" } },
        destination: { type: "string" },
        approvalRequirement: { type: "string" },
        limits: {
          type: "object",
          properties: {
            modelCalls: { type: "number" },
            timeoutSeconds: { type: "number" },
          },
        },
      },
      required: ["template", "invoiceIds", "destination"],
    },
  })
  @ApiResponse({ status: 201, description: "Run started successfully." })
  @UsePipes(new ZodValidationPipe(StartRunSchema))
  async startRun(
    @Body() startRunRequest: StartRunRequest,
    @Req() request: { operatorContext?: OperatorContext; id: string },
  ): Promise<StartRunResponse> {
    // The guard always sets the verified context; its absence is a server fault, never an allow.
    if (!request.operatorContext) {
      throw new InternalServerErrorException("Missing operator context");
    }

    const outcome = await this.gateway.postCommand(
      "/internal/runs",
      request.id,
      startRunRequest,
      StartRunResponseSchema,
      request.operatorContext,
    );
    if (outcome.success) {
      return outcome.data;
    }

    // A timeout does not prove the command failed: the outcome is unconfirmed.
    if (outcome.reason === "timeout") {
      throw new HttpException(
        {
          code: "outcome_unconfirmed",
          message: "The request timed out; the outcome is unconfirmed.",
        },
        504,
      );
    }
    if (outcome.reason === "unauthorized") {
      throw new HttpException(
        { code: "unauthorized", message: "Not authorized to start run" },
        403,
      );
    }
    if (outcome.reason === "bad_request") {
      throw new HttpException(
        { code: outcome.code ?? "bad_request", message: "Admission rejected" },
        400,
      );
    }
    throw new HttpException({ code: "upstream_unavailable", message: "Run start failed" }, 502);
  }
}
