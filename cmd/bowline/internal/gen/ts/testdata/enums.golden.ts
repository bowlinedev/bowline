import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type Query } from "@bowline/client";

export type Level = 1 | 10;

export interface Order {
  status: Status;
  level: Level;
  slug: Slug;
  history: Record<string, number>;
  levels: Level[];
}

export type Slug = string;

/** Status is the lifecycle state of an order. */
export type Status = "draft" | "sent" | "void";

export interface Errors {
  "get": BowlineError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  get: Query<Record<string, never>, Order>;
}

export const contract = {
  version: "1.2",
  hydrators: {},
  procedures: {
    "get": { kind: "query", method: "GET" },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
