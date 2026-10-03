import { Module } from "@nestjs/common";
import { GatewayClientModule } from "../gateway-client/gateway-client.module.js";
import { SecurityController } from "./security.controller.js";

@Module({ imports: [GatewayClientModule], controllers: [SecurityController] })
export class SecurityModule {}
