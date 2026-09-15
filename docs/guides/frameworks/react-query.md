# React Query

`@bowline/react-query` turns every query and mutation into TanStack Query option builders with the shared key shape.

source: examples/ledger/web/src/api.ts:54-54

```ts
export const bq = bowlineQuery(client);
```

source: examples/ledger/web/src/invoices.tsx:9-11

```tsx
  const list = useQuery(bq.invoices.list.queryOptions({ limit: 20 }));
  const invalidate = () => queryClient.invalidateQueries({ queryKey: bq.invoices.list.queryKey() });
  const create = useMutation({ ...bq.invoices.create.mutationOptions(), onSuccess: invalidate });
```

`queryOptions(input)` returns `{ queryKey, queryFn }` where the key is `[["invoices", "list"], input]` and the function forwards TanStack's abort signal to the client; `queryKey(input?)` alone builds the key for invalidation, and a call without input yields `[["invoices", "list"]]`, which matches every input as a prefix. `mutationOptions()` returns `{ mutationKey, mutationFn }`.

Proof: `pnpm --filter ledger-web test:e2e` drives the ledger app, which uses these helpers, through Playwright.

Gotchas: errors are `BowlineError` instances, so `error.code` and `error.issues` are typed; TanStack retries queries by default, including `INVALID_ARGUMENT` responses, so set `retry: false` or a code-aware `retry` function on the client. Subscriptions are not queries; use the client's `subscribe` directly and push into the cache with `setQueryData`.
