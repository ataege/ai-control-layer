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
    vendorId: z.string().optional(),
    invoiceIds: z.array(z.string()),
    destination: z.string(),
    approvalRequirement: z.string().optional(),
    limits: z.object({
      modelCalls: z.number().int().min(0),
      timeoutSeconds: z.number().int().min(0),
    }),
    versions: z.object({
      task: z.string(),
      policy: z.string(),
    }),
    rules: z.array(z.string()),
    expiresAt: z.string().datetime(),
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
    decision: z.enum(['allowed', 'denied', 'pending', 'blocked', 'redacted']).optional(),
    reasonCode: z.string().max(256).optional(),
    rule: z.string().max(256).optional(),
    revision: z.string().max(256).optional(),
    modelPurpose: z.string().max(256).optional(),
    correctionRoute: z.string().max(256).optional(),
    correctionCount: z.number().int().min(0).optional(),
    isReplay: z.boolean().optional(),
  }).passthrough(),
}) satisfies z.ZodType<SanitizedEvent>;

export const SanitizedEventsResponseSchema = z.object({
  events: z.array(SanitizedEventSchema),
  nextCursor: z.string().optional(),
});
