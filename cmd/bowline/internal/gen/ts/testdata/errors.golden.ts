import { createClient as create, type ClientOptions, type ContractRuntime, type Mutation, type TypedError, type UntypedError } from "@bowline/client";

/** InvoiceLocked is returned when an invoice cannot change. */
export interface InvoiceLocked {
  id: number;
  status: Status;
  since: Date;
}

export interface QuotaExceeded {
  limit: number;
}

export interface ID {
  id: number;
}

export interface Invoice {
  id: number;
}

export type Status = "draft" | "paid";

export interface Errors {
  "send": TypedError<"InvoiceLocked", InvoiceLocked> | UntypedError;
  "void": TypedError<"InvoiceLocked", InvoiceLocked> | TypedError<"QuotaExceeded", QuotaExceeded> | UntypedError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  send: Mutation<ID, Invoice, Errors["send"]>;
  "void": Mutation<ID, Invoice, Errors["void"]>;
}

export const contract = {
  version: "1.0",
  hydrators: {
    "InvoiceLocked": [
      { path: ["since"], kind: "timestamp" },
    ],
  },
  procedures: {
    "send": { kind: "mutation", method: "POST", errors: ["InvoiceLocked"] },
    "void": { kind: "mutation", method: "POST", errors: ["InvoiceLocked", "QuotaExceeded"] },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
