# SWR

`@bowlinedev/swr` turns every query into a `useQuery` hook built on `useSWR`, and every mutation into a `useMutation` hook built on `useSWRMutation`, using the shared key shape.

source: packages/swr/src/index.test.tsx:41-44

```tsx
    function Profile() {
      const { data } = swr.users.get.useQuery({ id: 1 });
      return <p>{data ? data.name : "loading"}</p>;
    }
```

`swr = bowlineSWR(client)`. `useQuery(input, config?, call?)` passes `[["users", "get"], input]` as the SWR key and the client call as the fetcher. This means `mutate(swr.users.get.key({ id: 1 }))` revalidates exactly that entry, and the same key works in a TanStack cache in a sibling app. `useMutation(config?, call?)` returns SWR's `{ trigger, data, error, isMutating }`, with `trigger(input)` typed from the contract.

Tests: `pnpm --filter @bowlinedev/swr test` renders both hooks against a fake client and checks the cache key.

Gotchas: SWR deduplicates by key, so two components asking for the same input share one request. A query whose input includes a timestamp or random value defeats that. Errors are `BowlineError`, and SWR retries on error by default. Set `shouldRetryOnError` to a code-aware function so that `INVALID_ARGUMENT` is not retried.
