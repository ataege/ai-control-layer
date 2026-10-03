import { Controller, Post, Get, Body, UsePipes, Req, InternalServerErrorException, HttpException, Param, NotFoundException, Query } from '@nestjs/common';
import { GatewayClientService } from '../gateway-client/gateway-client.service.js';
import { StartRunSchema } from './dto/start-run.dto.js';
import { RunViewSchema, SanitizedEventsResponseSchema } from './dto/run-view.dto.js';
import { ZodValidationPipe } from '../common/zod-validation.pipe.js';
import { StartRunRequest, StartRunResponse, OperatorContext, TaskFormOptions, RunView, SanitizedEvent } from '@workspace/contracts';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { FormOption } from './entities/form-option.entity.js';
import { z } from 'zod';
import { ApiTags, ApiOperation, ApiResponse, ApiBody } from '@nestjs/swagger';

const StartRunResponseSchema = z.object({
  runId: z.string(),
  passportId: z.string(),
});

@ApiTags('runs')
@Controller('runs')
export class RunsController {
  constructor(
    private readonly gateway: GatewayClientService,
    @InjectRepository(FormOption)
    private readonly formOptionRepo: Repository<FormOption>,
  ) {}

    const record = await this.formOptionRepo.findOne({
      where: { organizationId: req.operatorContext.organizationId },
    });
    if (!record) {
      return {
        templates: [],
        vendors: [],
        invoices: [],
        destinations: [],
        approvalRequirements: [],
        limits: { maxModelCalls: 0, maxTimeoutSeconds: 0 },
      };
    }
    return record.options;
  }

  @Post()
  @ApiOperation({ summary: 'Start a new run' })
  @ApiBody({
    schema: {
      type: 'object',
      properties: {
        template: { type: 'string' },
        vendorId: { type: 'string' },
        invoiceIds: { type: 'array', items: { type: 'string' } },
        destination: { type: 'string' },
        approvalRequirement: { type: 'string' },
        limits: {
          type: 'object',
          properties: {
            modelCalls: { type: 'number' },
            timeoutSeconds: { type: 'number' },
          },
        },
      },
      required: ['template', 'invoiceIds', 'destination'],
    },
  })
  @ApiResponse({ status: 201, description: 'Run started successfully.' })
  @UsePipes(new ZodValidationPipe(StartRunSchema))
  async startRun(
    @Body() dto: StartRunRequest,
    @Req() req: { operatorContext: OperatorContext; id: string }
  ): Promise<StartRunResponse> {
    if (!req.operatorContext) {
      throw new InternalServerErrorException('Missing operator context');
    }

    const outcome = await this.gateway.postCommand(
      '/internal/runs',
      req.id,
      dto,
      StartRunResponseSchema,
      req.operatorContext
    );

    if (outcome.success) {
      return outcome.data;
    } else {
      if (outcome.reason === 'timeout') {
        throw new HttpException({ code: 'timeout', message: 'The request timed out. The outcome is unconfirmed.' }, 504);
      }
      if (outcome.reason === 'timeout') {
      throw new HttpException({ code: 'timeout', message: 'The request timed out.' }, 504);
    }
    if (outcome.reason === 'timeout') {
      throw new HttpException({ code: 'timeout', message: 'The request timed out.' }, 504);
    }
    if (outcome.reason === 'unauthorized') {
      throw new HttpException({ code: 'unauthorized', message: 'Unauthorized' }, 403);
    }
    if (outcome.reason === 'bad_request' && outcome.code === 'not_found') {
      throw new NotFoundException({ code: 'not_found', message: 'Run not found' });
    }
    const statusCode = outcome.reason === 'bad_request' ? 400 : 500;
    throw new HttpException({ code: outcome.code || outcome.reason, message: 'Failed to get run events' }, statusCode);
  }
}
