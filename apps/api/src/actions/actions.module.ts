import { Module } from "@nestjs/common";
import { GatewayClientModule } from "../gateway-client/gateway-client.module.js";
import { ActionsController } from "./actions.controller.js";

@Module({ imports: [GatewayClientModule], controllers: [ActionsController] })
export class ActionsModule {}
