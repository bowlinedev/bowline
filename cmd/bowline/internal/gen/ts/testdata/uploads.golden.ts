import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type Upload } from "@bowline/client";

export interface AttachInput {
  invoiceId: number;
}

export interface Attachment {
  name: string;
  size: number;
}

export interface Errors {
  "attach": BowlineError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  /** Attach stores a file. */
  attach: Upload<AttachInput, Attachment>;
}

export const contract = {
  version: "1.0",
  hydrators: {},
  procedures: {
    "attach": { kind: "upload", method: "POST" },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
