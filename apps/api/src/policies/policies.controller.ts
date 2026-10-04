import {
  BadRequestException,
  Body,
  ConflictException,
  Controller,
  ForbiddenException,
  Get,
  Header,
  Post,
  Req,
  Res,
  ServiceUnavailableException,
} from "@nestjs/common";
import { ApiBody, ApiOperation, ApiResponse, ApiTags } from "@nestjs/swagger";
import type {
  PolicyReloadResponse,
  PolicyStatusResponse,
  OperatorContext,
} from "@workspace/contracts";
import requestContract from "@workspace/contracts/schemas/policy-reload-request.schema.json" with { type: "json" };
import responseContract from "@workspace/contracts/schemas/policy-reload-response.schema.json" with { type: "json" };
import type { Request, Response } from "express";
import { z } from "zod";
import { verifiedOperator } from "../gateway-client/gateway-response.js";
import { PoliciesService } from "./policies.service.js";
import { policyStatus } from "./policy-status.js";

const requestSchema = z.fromJSONSchema(requestContract as Parameters<typeof z.fromJSONSchema>[0]);
const responseSchema = z.fromJSONSchema(
  responseContract as unknown as Parameters<typeof z.fromJSONSchema>[0],
);
function reviewer(request: Request): OperatorContext {
  const operator = verifiedOperator(request);
  if (!operator.roles.includes("reviewer")) throw new ForbiddenException("Reviewer role required");
  return operator;
}

@ApiTags("policies")
@Controller("policies")
export class PoliciesController {
  constructor(private readonly policies: PoliciesService) {}

  @Post("reload")
  @Header("Cache-Control", "no-store")
  @ApiOperation({ summary: "Request a repository policy reload; only Go activates the revision" })
  @ApiBody({ schema: { type: "object", additionalProperties: false } })
  @ApiResponse({ status: 202, description: "Revision requested; activation is not yet confirmed." })
  @ApiResponse({ status: 200, description: "File unchanged; no revision created." })
  @ApiResponse({ status: 400, description: "Invalid body or policy/feed validation issues." })
  @ApiResponse({ status: 409, description: "A previous revision is still being validated." })
  async reload(
    @Body() body: unknown,
    @Req() request: Request,
    @Res({ passthrough: true }) response: Response,
  ): Promise<PolicyReloadResponse> {
    const operator = reviewer(request);
    if (!requestSchema.safeParse(body).success)
      throw new BadRequestException("Reload requires an empty object");
    const { outcome, feedRevision } = await this.policies.reload(operator.userId);
    if (!outcome.accepted) {
      if (outcome.pendingRevisionId !== undefined)
        throw new ConflictException({
          code: "revision_pending",
          message: "A previous revision is still being validated",
        });
      throw new BadRequestException({
        code: "policy_reload_rejected",
        message: "Policy validation failed",
        issues: outcome.issues,
      });
    }
    const result = outcome.unchanged
      ? { status: "unchanged", revisionId: outcome.revisionId }
      : {
          status: "requested",
          requestedRevisionId: outcome.revisionId,
          fileDigest: outcome.fileDigest,
          feedRevision,
        };
    const parsed = responseSchema.safeParse(result);
    if (!parsed.success) throw new ServiceUnavailableException("Invalid policy reload result");
    response.status(outcome.unchanged ? 200 : 202);
    return parsed.data as PolicyReloadResponse;
  }

  @Get("status")
  @Header("Cache-Control", "no-store")
  @ApiOperation({
    summary: "Read requested, validated and active catalog revisions; reviewer required",
  })
  async status(@Req() request: Request): Promise<PolicyStatusResponse> {
    reviewer(request);
    return policyStatus(await this.policies.pointer());
  }
}
