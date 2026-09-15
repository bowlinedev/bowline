import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type Mutation, type Query } from "@bowline/client";

export interface Item {
  name: string;
}

export interface Routing_ID {
  id: number;
}

export interface Sub_ID {
  id: number;
}

export interface Errors {
  "admin.purge": BowlineError;
  "get": BowlineError;
  "search": BowlineError;
  "sub.remove": BowlineError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  admin: {
    /** @deprecated use sub.remove */
    purge: Mutation<Routing_ID, Record<string, never>>;
  };
  /** Get fetches an item. */
  get: Query<Routing_ID, Item>;
  search: Query<Item, Item>;
  sub: {
    /** Remove deletes an item by ID. */
    remove: Mutation<Sub_ID, Record<string, never>>;
  };
}

export const contract = {
  version: "1.2",
  hydrators: {},
  procedures: {
    "admin.purge": { kind: "mutation", method: "POST" },
    "get": { kind: "query", method: "GET" },
    "search": { kind: "query", method: "POST" },
    "sub.remove": { kind: "mutation", method: "POST" },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
