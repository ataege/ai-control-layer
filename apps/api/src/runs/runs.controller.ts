import { Controller, Post, Get, Body, UsePipes, Req, InternalServerErrorException, HttpException, Param, NotFoundException, Query } from '@nestjs/common';
import type { Request } from 'express';
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

interface AuthenticatedRequest extends Request {
  operatorContext: OperatorContext;
  id: string;
}

@ApiTags('runs')
@Controller('runs')
export class RunsController {
  constructor(
    private readonly gateway: GatewayClientService,
    @InjectRepository(FormOption)
    private readonly formOptionRepo: Repository<FormOption>,
  ) {}
  @Get('options')
  @ApiOperation({ summary: 'Get task form options' })
  async getOptions(@Req() req: Request) {
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
    @Req() req: AuthenticatedRequest
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
    }

    if (outcome.reason === 'timeout') {
      throw new HttpException({ code: 'outcome_unconfirmed', message: 'The request timed out; the outcome is unconfirmed.' }, 504);
    }
    if (outcome.reason === 'unauthorized') {
      throw new HttpException({ code: 'unauthorized', message: 'Not authorized to start run' }, 403);
    }
    if (outcome.reason === 'bad_request') {
      throw new HttpException({ code: outcome.code ?? 'bad_request', message: 'Admission rejected' }, 400);
    }
    throw new HttpException({ code: 'upstream_unavailable', message: 'Run start failed' }, 502);
  }

  @Get(':id')
  @ApiOperation({ summary: 'Get run details' })
  async getRun(
    @Param('id') id: string,
    @Req() req: AuthenticatedRequest
  ): Promise<RunView> {
    const outcome = await this.gateway.fetchQuery(
      `/internal/runs/${encodeURIComponent(id)}`,
      RunViewSchema,
      req.operatorContext
    );

    if (outcome.success) {
      return outcome.data;
    }

    if (outcome.reason === 'unauthorized') {
      throw new HttpException({ code: 'unauthorized', message: 'Unauthorized' }, 403);
    }
    if (outcome.reason === 'bad_request' && outcome.code === 'not_found') {
      throw new NotFoundException({ code: 'not_found', message: 'Run not found' });
    }
    throw new HttpException({ code: outcome.code || outcome.reason, message: 'Failed to get run' }, 500);
  }

  @Get(':id/events')
  @ApiOperation({ summary: 'Get run events' })
  async getRunEvents(
    @Param('id') id: string,
    @Query('after') after: string | undefined,
    @Req() req: AuthenticatedRequest
  ): Promise<{ events: SanitizedEvent[]; nextCursor?: string }> {
    const query = after ? `?after=${encodeURIComponent(after)}` : '';
    const outcome = await this.gateway.fetchQuery(
      `/internal/runs/${encodeURIComponent(id)}/events${query}`,
      SanitizedEventsResponseSchema,
      req.operatorContext
    );

    if (outcome.success) {
      return outcome.data;
    }

    if (outcome.reason === 'unauthorized') {
      throw new HttpException({ code: 'unauthorized', message: 'Unauthorized' }, 403);
    }
    if (outcome.reason === 'bad_request' && outcome.code === 'not_found') {
      throw new NotFoundException({ code: 'not_found', message: 'Run not found' });
    }
    throw new HttpException({ code: outcome.code || outcome.reason, message: 'Failed to get run events' }, 500);
  }
}
