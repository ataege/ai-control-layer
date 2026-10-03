import { z } from 'zod';
import { RunView, SanitizedEvent } from '@workspace/contracts';

export const RunViewSchema = z.object({
  id: z.string().uuid(),
  status: z.enum(['pending', 'running', 'paused', 'failed', 'completed']),
  usage: z.object({
    modelCalls: z.number().int().min(0),
    cost: z.number().min(0),
  }),
  terminalReason: z.string().optional(),
  passport: z.object({
    id: z.string().uuid(),
    template: z.string(),
  }),
}) satisfies z.ZodType<RunView>;

export const SanitizedEventSchema = z.object({
  id: z.string().uuid(),
  type: z.string().max(256),
  timestamp: z.string().datetime(),
  details: z.object({
    message: z.string().max(2000).optional(),
    tool: z.string().max(256).optional(),
    action: z.string().max(256).optional(),
    decision: z.string().max(256).optional(),
  }).strict(),
}) satisfies z.ZodType<SanitizedEvent>;

export const SanitizedEventsResponseSchema = z.object({
  events: z.array(SanitizedEventSchema),
  nextCursor: z.string().optional(),
});
