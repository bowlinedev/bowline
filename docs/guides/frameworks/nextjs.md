# Next.js

A Next.js App Router page calls the API from a server component through `createServerClient`, forwarding the incoming request's cookies and authorization and tagging the fetch so a server action can revalidate it.

source: examples/nextjs/src/ledger.ts:7-14

```ts
export async function ledger(options: Partial<ServerClientOptions> = {}): Promise<Client> {
  const incoming = await headers();
  return createServerClient(createClient, {
    url: backend,
    request: { headers: incoming },
    ...options,
  });
}
```

source: examples/nextjs/app/page.tsx:7-9

```tsx
export default async function Page() {
  const api = await ledger({ next: { tags: ["invoices"] } });
  const page = await api.invoices.list({ limit: 20 });
```

source: examples/nextjs/app/actions.ts:11-22

```ts
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
```

`createServerClient` passes `cache` and `next` through to `fetch`, so tag revalidation works with no Next-specific package; the action's `.safe` call turns validation issues into data the form renders. Client components use `@bowlinedev/react-query` against `/api`, which `next.config.ts` rewrites to the Go server.

Proof: `cd examples/nextjs && pnpm build && pnpm test:e2e` asserts the raw HTML lists the seeded invoices before hydration, that the action returns issues, and that a created invoice appears through tag revalidation without a client refetch.

Gotchas: `headers()` is asynchronous in Next 15, hence the `await`. A server component fetch with no `cache` or `next` option is cached by Next's defaults on some routes, so tag it or set `dynamic`. Server actions must return plain data, which is why the action maps `BowlineError` to `{ ok, message, issues }` instead of throwing.
