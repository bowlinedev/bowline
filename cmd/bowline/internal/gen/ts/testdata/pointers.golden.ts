import { createClient as create, type ClientOptions, type ContractRuntime, type Query } from "@bowline/client";

export interface Doc {
  title: string | null;
  subtitle?: string;
  primary: Tag | null;
  tags: (Tag | null)[];
  byName: Record<string, Tag | null>;
  count?: number;
}

export interface Tag {
  label: string;
}

export interface Client {
  get: Query<Tag, Doc>;
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
