# Svelte

`@bowlinedev/svelte` gives every query a readable store with `loading`, `data`, `error`, and `refresh()`, and every mutation a `mutate` function with a `pending` store.

source: examples/sveltekit/src/lib/api.ts:1-5

```ts
import { bowlineStores } from "@bowlinedev/svelte";
import { createClient } from "./bowline.js";

export const client = createClient({ url: "/api" });
export const stores = bowlineStores(client);
```

source: packages/svelte/src/stores.test.ts:86-92

```ts
    const handle = bowlineStores(fakeClient([])).invoices.create.mutation();
    const pending: boolean[] = [];
    const stop = handle.state.subscribe((state) => pending.push(state.pending));
    const promise = handle.mutate({ total: "USD 2.00" });
    expect(pending).toEqual([false, true]);
    await expect(promise).resolves.toEqual({ id: 9, total: "USD 2.00" });
    expect(get(handle.state)).toEqual({ pending: false });
```

`query(input, { immediate })` fetches on first subscribe unless `immediate` is false, in which case the first `refresh()` does; `key(input?)` returns `[["invoices", "list"], input]` for anyone keeping a cache beside the stores.

Proof: `pnpm --filter @bowlinedev/svelte test`; the SvelteKit example under `examples/sveltekit` uses the stores in `+page.svelte`, see `sveltekit.md`.

Gotchas: a store created inside a component is created again on every remount; keep long-lived stores in a module. A `BowlineError` lands in `error` and other failures rethrow from `refresh` so a bug is never silently swallowed as a query error.
