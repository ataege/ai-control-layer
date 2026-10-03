import { z } from 'zod';

export const StartRunSchema = z.object({
  template: z.string().min(1),
  vendorId: z.string().optional(),
  invoiceIds: z.array(z.string()),
  destination: z.string().min(1),
  approvalRequirement: z.string().optional(),
  limits: z.object({
    modelCalls: z.number().int().positive().optional(),
    timeoutSeconds: z.number().int().positive().optional(),
  }).optional(),
});
