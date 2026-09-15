import { z } from "zod";

const Change = z.object({
  id: z.number().int(),
  at: z.date(),
  tags: z.array(z.string()),
});

const WatchInput = z.object({
  limit: z.number().int().min(1).max(100),
});

export const schemas = {
  Change,
  WatchInput,
} as const;

export const inputs = {
  "secret": WatchInput,
  "watch": WatchInput,
} as const;

export const errors = {
  Gone: z.object({
    id: z.number().int(),
  }),
} as const;
