# Vue

`@bowlinedev/vue` gives every query a `useQuery` composable with `data`, `error`, and `loading` refs that re-run when the input changes, and `queryOptions` shaped for `@tanstack/vue-query` when the app already uses it.

source: packages/vue/src/index.test.ts:100-112

```ts
      defineComponent({
        setup() {
          const options = bv.users.get.queryOptions({ id: 1 });
          const query = useTanstackQuery(options);
          return () =>
            h("p", { "data-testid": "state" }, [
              JSON.stringify(options.queryKey.value),
              "|",
              query.data.value?.name ?? "",
            ]);
        },
      }),
      { global: { plugins: [[VueQueryPlugin, { queryClient }]] } },
```

`bv = bowlineVue(client)`. `queryOptions(input)` accepts a value, a ref, or a getter and returns a computed `queryKey` plus a `queryFn`; `useQuery(input, { immediate })` is the TanStack-free composable; mutations offer `mutationOptions()` and `useMutation()` with `pending` and `error` refs.

Proof: `pnpm --filter @bowlinedev/vue test` covers both paths, including the resolved TanStack key `[["users", "get"], { id: 1 }]`.

Gotchas: `useQuery` must be called during `setup`, like any composable. A stale response from a superseded input is dropped, so rapid input changes never show an older result.
