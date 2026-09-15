import { createClient as create, type ClientOptions, type ContractRuntime, type DurationNs, type Query } from "@bowline/client";

export interface Event {
  at: Date;
  took: DurationNs;
  payload: unknown;
  addr: string;
  big: bigint;
  unsigned: bigint;
}

export interface Client {
  get: Query<Record<string, never>, Event>;
}

export const contract = {
  version: "1.0",
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
