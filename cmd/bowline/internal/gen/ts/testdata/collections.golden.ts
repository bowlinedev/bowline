import { createClient as create, type ClientOptions, type ContractRuntime, type Base64, type Query } from "@bowline/client";

export interface Cell {
  v: number;
}

export interface Grid {
  rows: Cell[][];
  fixed: number[];
  byKey: Record<string, Cell>;
  byInt: Record<string, string>;
  names?: string[];
  blob: Base64;
  ids: number[];
}

export interface Client {
  get: Query<Cell, Grid>;
}

export const contract = {
  version: "1.0",
  hydrators: {},
  procedures: {
    "get": { kind: "query", method: "GET" },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
