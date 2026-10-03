import {
  BadRequestException,
  Controller,
  ForbiddenException,
  Get,
  Query,
  Req,
  Res,
  ServiceUnavailableException,
} from "@nestjs/common";
import { ApiOperation, ApiTags } from "@nestjs/swagger";
import type { SecuritySummary, AssessmentPage, SecurityEventPage } from "@workspace/contracts";
import summaryContract from "@workspace/contracts/schemas/security-summary.schema.json" with { type: "json" };
import type { Request, Response } from "express";
import assessmentContract from "@workspace/contracts/schemas/assessment-record.schema.json" with { type: "json" };
import eventContract from "@workspace/contracts/schemas/safe-event.schema.json" with { type: "json" };
import { z } from "zod";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { gatewayData, verifiedOperator } from "../gateway-client/gateway-response.js";
import {
  AssessmentPageSchema,
  SecurityEventPageSchema,
  ExportQuerySchema,
  auditCsv,
} from "./audit-export.js";

const summarySchema = z.fromJSONSchema(summaryContract as Parameters<typeof z.fromJSONSchema>[0]);

@ApiTags("security")
@Controller("security")
export class SecurityController {
  constructor(private readonly gateway: GatewayClientService) {}

  @Get("export")
  @ApiOperation({
    summary: "Export one sanitized audit page as JSON or CSV; reviewer role required",
  })
  async export(
    @Query() query: Record<string, unknown>,
    @Req() request: Request,
    @Res({ passthrough: true }) response: Response,
  ): Promise<AssessmentPage | SecurityEventPage | string> {
    const operator = verifiedOperator(request);
    if (!operator.roles.includes("reviewer"))
      throw new ForbiddenException("Reviewer role required");
    const parsed = ExportQuerySchema.safeParse(query);
    if (!parsed.success) throw new BadRequestException("Invalid audit export query");
    const parameters = new URLSearchParams();
    if (parsed.data.after) parameters.set("cursor", parsed.data.after);
    if (parsed.data.limit) parameters.set("limit", parsed.data.limit);
    const schema = parsed.data.kind === "events" ? SecurityEventPageSchema : AssessmentPageSchema;
    const page = gatewayData(
      await this.gateway.getRead(
        `/internal/security/${parsed.data.kind}${parameters.size ? `?${parameters.toString()}` : ""}`,
        request.requestId,
        schema,
        operator,
      ),
    ) as AssessmentPage | SecurityEventPage;
    const records = "events" in page ? page.events : page.records;
    let previous = BigInt(parsed.data.after?.split(".")[3] ?? "0");
    for (const record of records) {
      const id = "eventId" in record ? record.eventId : record.assessmentId;
      if (
        BigInt(id) <= previous ||
        ("organizationId" in record && record.organizationId !== operator.organizationId)
      ) {
        throw new ServiceUnavailableException("Invalid gateway audit page");
      }
      previous = BigInt(id);
    }
    if (records.length > Number(parsed.data.limit ?? 100))
      throw new ServiceUnavailableException("Invalid gateway audit page limit");
    response.setHeader("Cache-Control", "no-store");
    if (parsed.data.format !== "csv") return page;
    response.setHeader("Content-Type", "text/csv; charset=utf-8");
    response.setHeader("Content-Disposition", `attachment; filename="${parsed.data.kind}.csv"`);
    response.setHeader("X-Next-Cursor", page.nextCursor);
    return auditCsv(
      records,
      Object.keys(
        parsed.data.kind === "events" ? eventContract.properties : assessmentContract.properties,
      ),
    );
  }

  @Get("summary")
  @ApiOperation({ summary: "Read the verified operator's organization security summary" })
  async summary(@Req() request: Request): Promise<SecuritySummary> {
    const operator = verifiedOperator(request);
    const summary = gatewayData(
      await this.gateway.getRead(
        "/internal/security/summary",
        request.requestId,
        summarySchema,
        operator,
      ),
    ) as SecuritySummary;
    if (summary.organizationId !== operator.organizationId) {
      throw new ServiceUnavailableException("Invalid gateway organization reference");
    }
    return summary;
  }
}
