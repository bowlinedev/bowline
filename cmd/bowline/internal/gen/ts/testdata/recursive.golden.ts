import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type Query } from "@bowline/client";

export interface Node {
  name: string;
  children: Node[];
  parent?: Node;
}

export interface Errors {
  "get": BowlineError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  get: Query<Node, Node>;
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
