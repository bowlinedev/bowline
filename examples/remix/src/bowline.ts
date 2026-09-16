import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type Mutation, type Query, type Subscription, type TypedError, type UntypedError, type Upload } from "@bowlinedev/client";

/** InvoiceLocked is returned when an invoice can no longer change. */
export interface InvoiceLocked {
  id: number;
  status: Status;
}

export interface AttachInput {
  invoiceId: number;
}

export interface Attachment {
  id: number;
  invoiceId: number;
  name: string;
  contentType: string;
  size: number;
  createdAt: Date;
  updatedAt: Date;
}

export interface CreateInvoiceInput {
  customerId: number;
  lines: Line[];
  note?: string;
}

export interface Customer {
  id: number;
  name: string;
  email: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface GetCustomerInput {
  id: number;
}

export interface GetInvoiceInput {
  id: number;
}

export interface HealthOutput {
  ok: boolean;
  version: string;
}

export interface Invoice {
  id: number;
  customerId: number;
  status: Status;
  total: string;
  lines: Line[];
  note?: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface Line {
  description: string;
  quantity: number;
  unitPrice: string;
}

export interface ListAttachmentsInput {
  invoiceId: number;
}

export interface ListInvoicesInput {
  cursor?: string;
  limit: number;
  status?: Status;
}

export interface Page<T> {
  items: T[];
  nextCursor?: string;
}

export interface SearchCustomersInput {
  query: string;
}

export type Status = "draft" | "sent" | "paid" | "void";

export interface VoidInvoiceInput {
  id: number;
}

export interface WatchInput {
  status?: Status;
}

export interface Errors {
  "customers.get": BowlineError;
  "customers.search": BowlineError;
  "health": BowlineError;
  "invoices.attach": BowlineError;
  "invoices.attachments": BowlineError;
  "invoices.create": BowlineError;
  "invoices.get": BowlineError;
  "invoices.list": BowlineError;
  "invoices.void": TypedError<"InvoiceLocked", InvoiceLocked> | UntypedError;
  "invoices.watch": BowlineError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  customers: {
    get: Query<GetCustomerInput, Customer>;
    /** Search finds customers whose name or email contains the query. */
    search: Query<SearchCustomersInput, Page<Customer>>;
  };
  health: Query<Record<string, never>, HealthOutput>;
  invoices: {
    /** Attach stores a file against an invoice. */
    attach: Upload<AttachInput, Attachment>;
    attachments: Query<ListAttachmentsInput, Page<Attachment>>;
    create: Mutation<CreateInvoiceInput, Invoice>;
    /** Get returns one invoice by ID. */
    get: Query<GetInvoiceInput, Invoice>;
    /** List returns a page of invoices, optionally filtered by status. */
    list: Query<ListInvoicesInput, Page<Invoice>>;
    /** Void cancels a draft or sent invoice. */
    "void": Mutation<VoidInvoiceInput, Invoice, Errors["invoices.void"]>;
    /** Watch streams every invoice change. */
    watch: Subscription<WatchInput, Invoice>;
  };
}

export const contract = {
  version: "1.2",
  hydrators: {
    "Attachment": [
      { path: ["createdAt"], kind: "timestamp" },
      { path: ["updatedAt"], kind: "timestamp" },
    ],
    "Customer": [
      { path: ["createdAt"], kind: "timestamp" },
      { path: ["updatedAt"], kind: "timestamp" },
    ],
    "Invoice": [
      { path: ["createdAt"], kind: "timestamp" },
      { path: ["updatedAt"], kind: "timestamp" },
    ],
    "Page<Attachment>": [
      { path: ["items", "*"], kind: { ref: "Attachment" } },
    ],
    "Page<Customer>": [
      { path: ["items", "*"], kind: { ref: "Customer" } },
    ],
    "Page<Invoice>": [
      { path: ["items", "*"], kind: { ref: "Invoice" } },
    ],
  },
  procedures: {
    "customers.get": { kind: "query", method: "GET", output: "Customer" },
    "customers.search": { kind: "query", method: "POST", output: "Page<Customer>" },
    "health": { kind: "query", method: "GET" },
    "invoices.attach": { kind: "upload", method: "POST", output: "Attachment" },
    "invoices.attachments": { kind: "query", method: "GET", output: "Page<Attachment>" },
    "invoices.create": { kind: "mutation", method: "POST", output: "Invoice" },
    "invoices.get": { kind: "query", method: "GET", output: "Invoice" },
    "invoices.list": { kind: "query", method: "GET", output: "Page<Invoice>" },
    "invoices.void": { kind: "mutation", method: "POST", output: "Invoice", errors: ["InvoiceLocked"] },
    "invoices.watch": { kind: "subscription", method: "GET", output: "Invoice" },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
