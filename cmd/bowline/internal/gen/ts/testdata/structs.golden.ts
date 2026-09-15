import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type Query } from "@bowline/client";

/** Address is where mail goes. */
export interface Address {
  /** Street line. */
  street: string;
  city: string;
}

export interface Person {
  name: string;
  age: number;
  email: string;
  home: Address;
  NoTag: string;
  inline: { x: number; };
}

export interface Errors {
  "get": BowlineError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  get: Query<Address, Person>;
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
