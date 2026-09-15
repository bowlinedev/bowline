import { z } from "zod";

const CreateInput = z.object({
  name: z.string().min(1),
});

const Created = z.object({
  id: z.number().int(),
});

export const schemas = {
  CreateInput,
  Created,
} as const;

export const inputs = {
  "create": CreateInput,
} as const;

export const errors = {
} as const;
