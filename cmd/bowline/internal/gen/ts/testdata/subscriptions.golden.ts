import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type Subscription, type TypedError, type UntypedError } from "@bowline/client";

export interface Gone {
  id: number;
}

export interface Change {
  id: number;
  at: Date;
  tags: string[];
}

export interface WatchInput {
  limit: number;
}

export interface Errors {
  "secret": BowlineError;
  "watch": TypedError<"Gone", Gone> | UntypedError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  secret: Subscription<WatchInput, Change>;
  /** Watch streams every change. */
  watch: Subscription<WatchInput, Change, Errors["watch"]>;
}

export const contract = {
  version: "1.0",
  hydrators: {
    "Change": [
      { path: ["at"], kind: "timestamp" },
    ],
  },
  procedures: {
    "secret": { kind: "subscription", method: "POST", output: "Change" },
    "watch": { kind: "subscription", method: "GET", output: "Change", errors: ["Gone"] },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
