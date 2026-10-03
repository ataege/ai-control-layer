import { Module } from "@nestjs/common";
import { GatewayClientModule } from "../gateway-client/gateway-client.module.js";
import { RunsController } from "./runs.controller.js";

@Module({
  imports: [GatewayClientModule],
  controllers: [RunsController],
})
export class RunsModule {}
