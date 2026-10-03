import { Module } from '@nestjs/common';
import { RunsController } from './runs.controller.js';
import { GatewayClientModule } from '../gateway-client/gateway-client.module.js';

import { TypeOrmModule } from '@nestjs/typeorm';
import { FormOption } from './entities/form-option.entity.js';

@Module({
  imports: [GatewayClientModule, TypeOrmModule.forFeature([FormOption])],
  controllers: [RunsController],
})
export class RunsModule {}
