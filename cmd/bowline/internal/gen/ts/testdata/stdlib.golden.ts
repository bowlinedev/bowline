import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type DurationNs, type Query } from "@bowline/client";

export interface Event {
  at: Date;
  took: DurationNs;
  payload: unknown;
  addr: string;
  big: bigint;
  unsigned: bigint;
}

export interface Errors {
  "get": BowlineError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  get: Query<Record<string, never>, Event>;
}

export const contract = {
  version: "1.2",
  hydrators: {
    "Event": [
      { path: ["at"], kind: "timestamp" },
      { path: ["big"], kind: "bigint" },
      { path: ["unsigned"], kind: "bigint" },
    ],
  },
  procedures: {
    "get": { kind: "query", method: "GET", output: "Event" },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
