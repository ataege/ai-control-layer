import { Controller, Get, Req, ServiceUnavailableException } from "@nestjs/common";
import { ApiOperation, ApiTags } from "@nestjs/swagger";
import type { SecuritySummary } from "@workspace/contracts";
import summaryContract from "@workspace/contracts/schemas/security-summary.schema.json" with { type: "json" };
import type { Request } from "express";
import { z } from "zod";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { gatewayData, verifiedOperator } from "../gateway-client/gateway-response.js";

const summarySchema = z.fromJSONSchema(summaryContract as Parameters<typeof z.fromJSONSchema>[0]);

@ApiTags("security")
@Controller("security")
export class SecurityController {
  constructor(private readonly gateway: GatewayClientService) {}

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
