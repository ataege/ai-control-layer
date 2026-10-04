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
  Passport,
} from "@workspace/contracts";
import type { Request } from "express";
import { z } from "zod";
import { ZodValidationPipe } from "../common/zod-validation.pipe.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { StartRunRequestDto, StartRunResponseDto, StartRunSchema } from "./dto/start-run.dto.js";
import startResponseContract from "@workspace/contracts/schemas/start-run-response.schema.json" with { type: "json" };
import passportContract from "@workspace/contracts/schemas/passport.schema.json" with { type: "json" };
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

const PassportSchema = z.fromJSONSchema(passportContract as Parameters<typeof z.fromJSONSchema>[0]);

// The admission rejections whose fixed explanation names the scope or limit to change (GO-13,
// lane w3's fixed texts). Any other code, even a valid X-13 one, keeps the generic message.
const ADMISSION_REASON_CODES: ReadonlySet<string> = new Set([
  "resource_out_of_scope",
  "destination_not_allowed",
  "template_not_allowed",
  "limit_not_allowed",
  "invalid_arguments",
]);
const MAXIMUM_EXPLANATION_LENGTH = 300;

/** A 400 from admission with a contract reason code and a bounded, printable explanation. */
function isAdmissionRejection(
  code: string | undefined,
  message: string | undefined,
): message is string {
  return (
    code !== undefined &&
    ADMISSION_REASON_CODES.has(code) &&
    message !== undefined &&
    message.length > 0 &&
    message.length <= MAXIMUM_EXPLANATION_LENGTH &&
    // eslint-disable-next-line no-control-regex
    !/[\u0000-\u001f\u007f]/.test(message)
  );
}

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

  @Get(":id/passport")
  @ApiOperation({ summary: "Read the stored immutable passport of an authorized run" })
  async passport(@Param("id") runId: string, @Req() request: Request): Promise<Passport> {
    const operator = verifiedOperator(request);
    requireRecordId(runId);
    const passport = gatewayData(
      await this.gateway.getRead(
        `/internal/runs/${runId}/passport`,
        request.requestId,
        PassportSchema,
        operator,
      ),
    ) as Passport;
    if (passport.runId !== runId || passport.organizationId !== operator.organizationId)
      throw new ServiceUnavailableException("Invalid gateway passport reference");
    return passport;
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
          .refine(
            (cursor) =>
              /^(0|[1-9][0-9]{0,18})$/.test(cursor) && BigInt(cursor) <= 9223372036854775807n,
          )
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
  @ApiOperation({
    summary: "Start a new run",
    description:
      "Validates the X-07 body strictly, takes actor and organization only from the session and forwards the command to Go's admission.",
  })
  @ApiBody({ type: StartRunRequestDto })
  @ApiResponse({
    status: 201,
    type: StartRunResponseDto,
    description: "Admitted: Go stored the passport, run and job.",
  })
  @ApiResponse({
    status: 400,
    description:
      "Invalid body, or an admission rejection with its X-13 reason code and explanation; no passport.",
  })
  @ApiResponse({ status: 401, description: "No valid session." })
  @ApiResponse({ status: 503, description: "Admission could not be decided; no run was started." })
  @ApiResponse({
    status: 504,
    description: "The gateway did not answer in time; the outcome is unconfirmed.",
  })
  @UsePipes(new ZodValidationPipe(StartRunSchema))
  async startRun(
    @Body() startRunRequest: StartRunRequest,
    @Req() request: Request,
  ): Promise<StartRunResponse> {
    return this.admit("/internal/runs", startRunRequest, request);
  }

  // A judge run has a passport and a run but no agent: nothing is dispatched for it, and judge
  // evaluations spend its security allowance. Same body, response, guard and error mapping as a
  // run; only the gateway route differs (Go's judge admission).
  @Post("judge")
  @ApiOperation({
    summary: "Start a judge run: a passport and run with no agent",
    description:
      "Validates the same X-07 body strictly and forwards it to Go's judge admission. The run has a passport but no agent job, so no model call is made for it; evaluations through POST /api/control/evaluate spend its security allowance.",
  })
  @ApiBody({ type: StartRunRequestDto })
  @ApiResponse({
    status: 201,
    type: StartRunResponseDto,
    description: "Admitted: Go stored the passport and the run, with no agent job.",
  })
  @ApiResponse({
    status: 400,
    description:
      "Invalid body, or an admission rejection with its X-13 reason code and explanation; no passport.",
  })
  @ApiResponse({ status: 401, description: "No valid session." })
  @ApiResponse({ status: 503, description: "Admission could not be decided; no run was started." })
  @ApiResponse({
    status: 504,
    description: "The gateway did not answer in time; the outcome is unconfirmed.",
  })
  @UsePipes(new ZodValidationPipe(StartRunSchema))
  async startJudgeRun(
    @Body() startRunRequest: StartRunRequest,
    @Req() request: Request,
  ): Promise<StartRunResponse> {
    return this.admit("/internal/judge-runs", startRunRequest, request);
  }

  /** Forwards one validated admission command to Go and maps its answer. */
  private async admit(
    gatewayPath: string,
    startRunRequest: StartRunRequest,
    request: Request,
  ): Promise<StartRunResponse> {
    const operator = verifiedOperator(request);
    const outcome = await this.gateway.postCommand(
      gatewayPath,
      request.requestId,
      startRunRequest,
      StartRunResponseSchema,
      operator,
      { keepErrorMessage: true },
    );
    // An admission rejection keeps Go's reason code and its fixed explanation, so the operator
    // sees which scope or limit must change; NestJS never narrows the request itself (API-11).
    if (
      !outcome.success &&
      outcome.statusCode === 400 &&
      isAdmissionRejection(outcome.code, outcome.message)
    ) {
      throw new HttpException({ code: outcome.code, message: outcome.message }, 400);
    }
    return gatewayData(outcome, true) as StartRunResponse;
  }
}
