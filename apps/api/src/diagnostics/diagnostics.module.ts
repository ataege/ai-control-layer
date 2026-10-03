import { Module } from "@nestjs/common";
import { GatewayClientModule } from "../gateway-client/gateway-client.module.js";
import { DiagnosticsController } from "./diagnostics.controller.js";

@Module({
  imports: [GatewayClientModule],
  controllers: [DiagnosticsController],
})
export class DiagnosticsModule {}
