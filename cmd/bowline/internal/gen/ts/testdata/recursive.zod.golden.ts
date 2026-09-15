import { z } from "zod";

const Node = z.object({
  name: z.string(),
  get children() {
    return z.array(Node);
  },
  get parent() {
    return Node.optional();
  },
});

export const schemas = {
  Node,
} as const;

export const inputs = {
  "get": Node,
} as const;

export const errors = {
} as const;
