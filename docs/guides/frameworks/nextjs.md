# Next.js

A Next.js App Router page calls the API from a server component through `createServerClient`. It forwards the incoming request's cookies and authorization header, and tags the fetch so a server action can revalidate it.

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

`createServerClient` passes `cache` and `next` through to `fetch`, so tag revalidation works without a Next-specific package. The action's `.safe` call turns validation issues into data that the form can render. Client components use `@bowlinedev/react-query` against `/api`, which `next.config.ts` rewrites to the Go server.

Tests: `cd examples/nextjs && pnpm build && pnpm test:e2e` checks that the raw HTML lists the seeded invoices before hydration, that the action returns issues, and that a created invoice appears through tag revalidation without a client-side refetch.

Gotchas: `headers()` is asynchronous in Next 15, which is why it is awaited. A server component fetch with no `cache` or `next` option gets cached by Next's defaults on some routes, so tag it or set `dynamic`. Server actions have to return plain data, which is why the action maps `BowlineError` to `{ ok, message, issues }` rather than throwing.
