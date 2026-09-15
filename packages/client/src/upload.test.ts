import { expect, test, vi } from "vitest";
import { createClient } from "./client.js";
import type { ContractRuntime, Upload } from "./types.js";

const contract: ContractRuntime = {
  version: "1.0",
  hydrators: {},
  procedures: { "invoices.attach": { kind: "upload", method: "POST" } },
};

interface Client {
  invoices: { attach: Upload<{ invoiceId: number }, { name: string; size: number }> };
}

test("uploads send the input part first and the file part second", async () => {
  let seen: RequestInit | undefined;
  const fetchFn = vi.fn(async (_url: string | URL | Request, init?: RequestInit) => {
    seen = init;
    return new Response(JSON.stringify({ name: "receipt.pdf", size: 4 }), { status: 200 });
  }) as unknown as typeof fetch;
  const client = createClient(contract, { url: "http://api.test", fetch: fetchFn }) as Client;
  const file = new File(["%PDF"], "receipt.pdf", { type: "application/pdf" });
  const out = await client.invoices.attach({ invoiceId: 3 }, file);
  expect(out).toEqual({ name: "receipt.pdf", size: 4 });
  expect(seen?.method).toBe("POST");
  expect(new Headers(seen?.headers).get("content-type")).toBeNull();
  const form = seen?.body as FormData;
  const entries = [...form.entries()];
  expect(entries.map(([k]) => k)).toEqual(["input", "file"]);
  const input = entries[0]?.[1] as Blob;
  expect(input.type).toBe("application/json");
  expect(await input.text()).toBe('{"invoiceId":3}');
  const sent = entries[1]?.[1] as File;
  expect(sent.name).toBe("receipt.pdf");
  expect(client.invoices.attach.kind).toBe("upload");
});
