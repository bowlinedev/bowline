import { createClient as create, type ClientOptions, type ContractRuntime, type BowlineError, type Mutation, type Query } from "@bowline/client";

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

export interface Errors {
  "customers.get": BowlineError;
  "customers.search": BowlineError;
  "health": BowlineError;
  "invoices.create": BowlineError;
  "invoices.get": BowlineError;
  "invoices.list": BowlineError;
  "invoices.void": BowlineError;
}

export type ProcedureError<P extends keyof Errors> = Errors[P];

export interface Client {
  customers: {
    get: Query<GetCustomerInput, Customer>;
    search: Query<SearchCustomersInput, Page<Customer>>;
  };
  health: Query<Record<string, never>, HealthOutput>;
  invoices: {
    create: Mutation<CreateInvoiceInput, Invoice>;
    /** Get returns one invoice by ID. */
    get: Query<GetInvoiceInput, Invoice>;
    list: Query<ListInvoicesInput, Page<Invoice>>;
    "void": Mutation<VoidInvoiceInput, Invoice>;
  };
}

export const contract = {
  version: "1.0",
  hydrators: {
    "Customer": [
      { path: ["createdAt"], kind: "timestamp" },
      { path: ["updatedAt"], kind: "timestamp" },
    ],
    "Invoice": [
      { path: ["createdAt"], kind: "timestamp" },
      { path: ["updatedAt"], kind: "timestamp" },
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
    "invoices.create": { kind: "mutation", method: "POST", output: "Invoice" },
    "invoices.get": { kind: "query", method: "GET", output: "Invoice" },
    "invoices.list": { kind: "query", method: "GET", output: "Page<Invoice>" },
    "invoices.void": { kind: "mutation", method: "POST", output: "Invoice" },
  },
} satisfies ContractRuntime;

export function createClient(options: ClientOptions): Client {
  return create(contract, options) as Client;
}
