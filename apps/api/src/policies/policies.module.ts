import { Module } from "@nestjs/common";
import { GatewayClientModule } from "../gateway-client/gateway-client.module.js";
import { PoliciesController } from "./policies.controller.js";
import { PoliciesService } from "./policies.service.js";
import { PolicyCatalogController } from "./policy-catalog.controller.js";

@Module({
  imports: [GatewayClientModule],
  controllers: [PoliciesController, PolicyCatalogController],
  providers: [PoliciesService],
})
export class PoliciesModule {}
