import { z } from "zod";

const Input = z.object({
  name: z.string(),
  count: z.number().int(),
  small: z.number().int(),
  big: z.number().int(),
  ratio: z.number(),
  on: z.boolean(),
});

const Output = z.object({
  echo: z.string(),
});

export const schemas = {
  Input,
  Output,
} as const;

export const inputs = {
  "echo": Input,
} as const;

export const errors = {
} as const;
