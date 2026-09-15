import { z } from "zod";

const Address = z.object({
  city: z.string(),
});

const Level = z.union([z.literal(1), z.literal(10)]);

const Slug = z.string();

const Status = z.enum(["draft", "sent"]);

const Sample = z.object({
  name: z.string().min(1),
  email: z.string().email(),
  age: z.number().int().min(0),
  big: z.bigint(),
  ratio: z.number(),
  active: z.boolean(),
  status: Status,
  level: Level,
  slug: Slug,
  since: z.date(),
  timeout: z.number().int(),
  blob: z.string(),
  raw: z.unknown(),
  tags: z.array(z.string()),
  counts: z.record(z.string(), z.number().int()),
  home: Address,
  nick: z.string().optional(),
  ratings: z.array(Level),
});

export const schemas = {
  Address,
  Level,
  Sample,
  Slug,
  Status,
} as const;

export const inputs = {
  "get": z.object({}),
} as const;

export const errors = {
} as const;
