"use server";

import type { Issue } from "@bowlinedev/client";
import { revalidateTag } from "next/cache";
import { ledger } from "../src/ledger";

export type CreateResult =
  | { ok: true; id: number }
  | { ok: false; message: string; issues: Issue[] };

export async function createInvoice(description: string, quantity: number): Promise<CreateResult> {
  const api = await ledger();
  const result = await api.invoices.create.safe({
    customerId: 1,
    lines: [{ description, quantity, unitPrice: "USD 10.00" }],
  });
  if (!result.ok) {
    return { ok: false, message: result.error.message, issues: result.error.issues };
  }
  revalidateTag("invoices");
  return { ok: true, id: result.value.id };
}
