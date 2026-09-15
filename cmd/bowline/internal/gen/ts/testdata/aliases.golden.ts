import { createClient as create, type ClientOptions, type ContractRuntime, type Query } from "@bowline/client";

export interface Holder {
  a: Real;
  b: Real[];
}

export interface Real {
  n: number;
}

export interface Client {
  get: Query<Real, Holder>;
}

export const contract = {
  version: "0.1",
  hydrators: {},
  procedures: {
    "get": { kind: "query", method: "GET" },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
