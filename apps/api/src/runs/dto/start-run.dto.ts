import { z } from "zod";

export const StartRunSchema = z
  .object({
    template: z.string().min(1).max(256),
    vendorId: z.string().max(256).optional(),
    invoiceIds: z.array(z.string().max(256)).max(100),
    destination: z.string().min(1).max(256),
    approvalRequirement: z.string().max(256).optional(),
    limits: z
      .object({
        modelCalls: z.number().int().positive().max(1000).optional(),
        timeoutSeconds: z.number().int().positive().max(3600).optional(),
      })
      .strict()
      .optional(),
  })
  .strict();
