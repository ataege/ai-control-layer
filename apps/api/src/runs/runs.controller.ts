import {
  Body,
  Controller,
  HttpException,
  InternalServerErrorException,
  Post,
  Req,
  UsePipes,
} from "@nestjs/common";
import { ApiBody, ApiOperation, ApiResponse, ApiTags } from "@nestjs/swagger";
import type { OperatorContext, StartRunRequest, StartRunResponse } from "@workspace/contracts";
import { z } from "zod";
import { ZodValidationPipe } from "../common/zod-validation.pipe.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { StartRunSchema } from "./dto/start-run.dto.js";

const StartRunResponseSchema = z.object({
  runId: z.string(),
  passportId: z.string(),
});

// The run and event reads wait on the open `read path` item, and the task form options on
// `form options`; this controller exposes only the start-run command (API-11).
@ApiTags("runs")
@Controller("runs")
export class RunsController {
  constructor(private readonly gateway: GatewayClientService) {}

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
