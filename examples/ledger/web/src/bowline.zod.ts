import { z } from "zod";

const AttachInput = z.object({
  invoiceId: z.number().int(),
});

const Attachment = z.object({
  id: z.number().int(),
  invoiceId: z.number().int(),
  name: z.string(),
  contentType: z.string(),
  size: z.number().int(),
  createdAt: z.date(),
  updatedAt: z.date(),
});

const Line = z.object({
  description: z.string().min(1).max(200),
  quantity: z.number().int().min(1),
  unitPrice: z.string(),
});

const CreateInvoiceInput = z.object({
  customerId: z.number().int(),
  lines: z.array(Line).min(1),
  note: z.string().max(500).optional(),
});

const Customer = z.object({
  id: z.number().int(),
  name: z.string().min(1),
  email: z.string().min(1).email(),
  createdAt: z.date(),
  updatedAt: z.date(),
});

const GetCustomerInput = z.object({
  id: z.number().int(),
});

const GetInvoiceInput = z.object({
  id: z.number().int(),
});

const HealthOutput = z.object({
  ok: z.boolean(),
  version: z.string(),
});

const Status = z.enum(["draft", "sent", "paid", "void"]);

const Invoice = z.object({
  id: z.number().int(),
  customerId: z.number().int(),
  status: Status,
  total: z.string(),
  lines: z.array(Line),
  note: z.string().optional(),
  createdAt: z.date(),
  updatedAt: z.date(),
});

const ListAttachmentsInput = z.object({
  invoiceId: z.number().int(),
});

const ListInvoicesInput = z.object({
  cursor: z.string().optional(),
  limit: z.number().int().min(1).max(100),
  status: Status.optional(),
});

const Page = <T extends z.ZodTypeAny>(t: T) =>
  z.object({
    items: z.array(t),
    nextCursor: z.string().optional(),
  });

const SearchCustomersInput = z.object({
  query: z.string().min(2),
});

const VoidInvoiceInput = z.object({
  id: z.number().int(),
});

const WatchInput = z.object({
  status: Status.optional(),
});

export const schemas = {
  AttachInput,
  Attachment,
  CreateInvoiceInput,
  Customer,
  GetCustomerInput,
  GetInvoiceInput,
  HealthOutput,
  Invoice,
  Line,
  ListAttachmentsInput,
  ListInvoicesInput,
  Page,
  SearchCustomersInput,
  Status,
  VoidInvoiceInput,
  WatchInput,
} as const;

export const inputs = {
  "customers.get": GetCustomerInput,
  "customers.search": SearchCustomersInput,
  "health": z.object({}),
  "invoices.attach": AttachInput,
  "invoices.attachments": ListAttachmentsInput,
  "invoices.create": CreateInvoiceInput,
  "invoices.get": GetInvoiceInput,
  "invoices.list": ListInvoicesInput,
  "invoices.void": VoidInvoiceInput,
  "invoices.watch": WatchInput,
} as const;

export const errors = {
  InvoiceLocked: z.object({
    id: z.number().int(),
    status: Status,
  }),
} as const;
