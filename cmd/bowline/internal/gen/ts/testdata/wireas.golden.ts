import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type Query } from "@bowline/client";

export interface Envelope {
  kind: string;
}

export interface Line {
  price: string;
  origin: number[];
  meta: Envelope;
  prices: string[];
}

export interface Errors {
  "get": BowlineError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  get: Query<Record<string, never>, Line>;
}

export const contract = {
  version: "1.1",
  hydrators: {},
  procedures: {
    "get": { kind: "query", method: "GET" },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
