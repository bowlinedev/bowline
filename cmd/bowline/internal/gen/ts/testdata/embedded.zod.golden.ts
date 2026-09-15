import { z } from "zod";

const Audit = z.object({
  createdBy: z.string(),
  version: z.number().int(),
});

const Named = z.object({
  name: z.string(),
});

const Record = z.object({
  id: z.number().int(),
  createdBy: z.string(),
  version: z.number().int(),
  named: Named.nullable(),
  extra: z.string(),
});

export const schemas = {
  Audit,
  Named,
  Record,
} as const;

export const inputs = {
  "get": Audit,
} as const;

export const errors = {
} as const;
