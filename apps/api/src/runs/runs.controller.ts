import {
  Body,
  BadRequestException,
  Controller,
  Get,
  HttpCode,
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
  StartRunRequest,
  StartRunResponse,
  RunEventsPage,
  RunState,
  RunUsage,
  ReportView,
  TaskFormOptions,
} from "@workspace/contracts";
import type { Request } from "express";
import { z } from "zod";
import { ZodValidationPipe } from "../common/zod-validation.pipe.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { StartRunSchema } from "./dto/start-run.dto.js";
import startResponseContract from "@workspace/contracts/schemas/start-run-response.schema.json" with { type: "json" };
import optionsContract from "@workspace/contracts/schemas/task-form-options.schema.json" with { type: "json" };
import { RunEventsSchema } from "./run-events.schema.js";
import { RunStateSchema, RunUsageSchema } from "./run-read.schemas.js";
import { ReportViewSchema } from "./report-view.schema.js";
import {
  gatewayData,
  requireRecordId,
  verifiedOperator,
} from "../gateway-client/gateway-response.js";

const StartRunResponseSchema = z.fromJSONSchema(
  startResponseContract as Parameters<typeof z.fromJSONSchema>[0],
);
const TaskFormOptionsSchema = z.fromJSONSchema(
  optionsContract as Parameters<typeof z.fromJSONSchema>[0],
);

@ApiTags("runs")
@Controller("runs")
export class RunsController {
  constructor(private readonly gateway: GatewayClientService) {}

  // Register before :id so "options" is never interpreted as a run reference.
  @Get("options")
  @ApiOperation({ summary: "Read organization-scoped task choices from the active Go catalog" })
  @ApiResponse({ status: 200, description: "The unchanged shared TaskFormOptions response." })
  @ApiResponse({ status: 401, description: "Verified session and membership required." })
  @ApiResponse({ status: 503, description: "Catalog or gateway unavailable; no default options." })
  async options(@Req() request: Request): Promise<TaskFormOptions> {
    return gatewayData(
      await this.gateway.getRead(
        "/internal/task-options",
        request.requestId,
        TaskFormOptionsSchema,
        verifiedOperator(request),
      ),
    ) as TaskFormOptions;
  }

  @Get(":id/reports/:reportId")
  @ApiOperation({ summary: "Read a stored report with its server classification and source trail" })
  async report(
    @Param("id") runId: string,
    @Param("reportId") reportId: string,
    @Req() request: Request,
  ): Promise<ReportView> {
    const operator = verifiedOperator(request);
    requireRecordId(runId);
    requireRecordId(reportId);
    const report = gatewayData(
      await this.gateway.getRead(
        `/internal/runs/${runId}/reports/${reportId}`,
        request.requestId,
        ReportViewSchema,
        operator,
      ),
    ) as ReportView;
    if (report.runId !== runId || report.reportId !== reportId) {
      throw new ServiceUnavailableException("Invalid gateway report reference");
    }
    return report;
  }

  @Get(":id")
  @ApiOperation({ summary: "Read the authorized run state, including stored result references" })
  async state(@Param("id") runId: string, @Req() request: Request): Promise<RunState> {
    const operator = verifiedOperator(request);
    requireRecordId(runId);
    const state = gatewayData(
      await this.gateway.getRead(
        `/internal/runs/${runId}`,
        request.requestId,
        RunStateSchema,
        operator,
      ),
    ) as RunState;
    if (state.runId !== runId)
      throw new ServiceUnavailableException("Invalid gateway run reference");
    return state;
  }

  @Get(":id/usage")
  @ApiOperation({ summary: "Read the authorized run's recorded usage" })
  async usage(@Param("id") runId: string, @Req() request: Request): Promise<RunUsage> {
    const operator = verifiedOperator(request);
    requireRecordId(runId);
    const usage = gatewayData(
      await this.gateway.getRead(
        `/internal/runs/${runId}/usage`,
        request.requestId,
        RunUsageSchema,
        operator,
      ),
    ) as RunUsage;
    if (usage.runId !== runId)
      throw new ServiceUnavailableException("Invalid gateway run reference");
    return usage;
  }

  @Post(":id/cancel")
  @HttpCode(200)
  @ApiOperation({
    summary: "Request cancellation: stops future dispatches and preserves committed effects",
  })
  @ApiResponse({
    status: 200,
    description: "Go's recorded run state after the cancellation request.",
  })
  async cancel(
    @Param("id") runId: string,
    @Body() body: unknown,
    @Req() request: Request,
  ): Promise<RunState> {
    const operator = verifiedOperator(request);
    requireRecordId(runId);
    if (!z.strictObject({}).optional().safeParse(body).success) {
      throw new BadRequestException("Cancellation accepts no command fields");
    }
    const state = gatewayData(
      await this.gateway.postCommand(
        `/internal/runs/${runId}/cancel`,
        request.requestId,
        {},
        RunStateSchema,
        operator,
      ),
      true,
    ) as RunState;
    if (state.runId !== runId)
      throw new ServiceUnavailableException("Invalid gateway run reference");
    return state;
  }

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
    @Req() request: Request,
  ): Promise<StartRunResponse> {
    const operator = verifiedOperator(request);
    return gatewayData(
      await this.gateway.postCommand(
        "/internal/runs",
        request.requestId,
        startRunRequest,
        StartRunResponseSchema,
        operator,
      ),
      true,
    ) as StartRunResponse;
  }
}
