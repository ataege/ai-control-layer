import { Controller, ForbiddenException, Get, Header, Req } from "@nestjs/common";
import { ApiOperation, ApiTags } from "@nestjs/swagger";
import type { CatalogStatus } from "@workspace/contracts";
import catalogContract from "@workspace/contracts/schemas/catalog-status.schema.json" with { type: "json" };
import type { Request } from "express";
import { z } from "zod";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { gatewayData, verifiedOperator } from "../gateway-client/gateway-response.js";

const catalogSchema = z.fromJSONSchema(catalogContract as Parameters<typeof z.fromJSONSchema>[0]);

@ApiTags("policies")
@Controller("policies")
export class PolicyCatalogController {
  constructor(private readonly gateway: GatewayClientService) {}

  @Get("catalog")
  @Header("Cache-Control", "no-store")
  @ApiOperation({
    summary: "Read the active control catalog as the gateway enforces it; reviewer required",
  })
  async catalog(@Req() request: Request): Promise<CatalogStatus> {
    const operator = verifiedOperator(request);
    if (!operator.roles.includes("reviewer"))
      throw new ForbiddenException("Reviewer role required");
    // The gateway is the authority for the active revision; a failed or malformed answer is never
    // turned into an empty catalog.
    return gatewayData(
      await this.gateway.getRead(
        "/internal/catalog/active",
        request.requestId,
        catalogSchema,
        operator,
      ),
    ) as CatalogStatus;
  }
}
