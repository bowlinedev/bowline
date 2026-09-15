import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type Query } from "@bowline/client";

export interface Audit {
  createdBy: string;
  version: number;
}

export interface Named {
  name: string;
}

export interface Record {
  id: number;
  createdBy: string;
  version: number;
  named: Named | null;
  extra: string;
}

export interface Errors {
  "get": BowlineError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  get: Query<Audit, Record>;
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
