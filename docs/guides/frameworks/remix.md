# React Router

In React Router's framework mode, a `loader` calls the API on the server through `createServerClient` with the incoming `Request`, and the component uses `@bowlinedev/react-query` for client-side updates.

source: examples/remix/src/ledger.ts:6-8

```ts
export function ledger(request: Request): Client {
  return createServerClient(createClient, { url: backend, request });
}
```

source: examples/remix/app/routes/home.tsx:9-24

```tsx
export async function loader({ request }: Route.LoaderArgs) {
  const page = await ledger(request).invoices.list({ limit: 20 });
  return { invoices: page.items };
}

export default function Home({ loaderData }: Route.ComponentProps) {
  const revalidator = useRevalidator();
  const queryClient = useQueryClient();
  const health = useQuery(bq.health.queryOptions());
  const create = useMutation({
    ...bq.invoices.create.mutationOptions(),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: bq.invoices.list.queryKey() });
      await revalidator.revalidate();
    },
  });
```

A resource route at `app/routes/api.ts` proxies `/api/*` to the Go server so the browser client stays same-origin in dev and production.

Proof: `cd examples/remix && pnpm build && pnpm test:e2e` asserts the server-rendered list, a client-side create, and validation issues.

Gotchas: React Router's default server entry treats bot user agents as non-hydrating and Playwright's headless Chrome is one of them; the example's Playwright config sets a browser user agent. Loader data and query data are two caches; after a mutation invalidate the query and revalidate the loader, as above.
