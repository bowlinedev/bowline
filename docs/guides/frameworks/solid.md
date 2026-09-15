# Solid

`@bowline/solid` wraps every query in `createResource` and every mutation in an action with a pending and error signal.

source: examples/astro/src/components/Invoices.tsx:5-9

```tsx
const ledger = bowlineSolid(client);

export default function Invoices() {
  const [list, { refetch }] = ledger.invoices.list.resource({ limit: 20 });
  const [create, state] = ledger.invoices.create.action();
```

`resource(input)` accepts a plain input or an accessor; with an accessor the resource refetches when the input changes. `action()` returns `[mutate, state]` where `mutate(input)` resolves with the output and `state()` carries `pending` and a `BowlineError` when the call failed.

Proof: `pnpm --filter @bowline/solid test`; the Astro example's island under `examples/astro` uses both helpers and its Playwright suite drives them, see `astro.md`.

Gotchas: a resource created during server rendering runs its fetcher on the server, where a relative URL such as `/api` has no origin; render the island client-only or give the server an absolute URL. `refetch()` after a mutation is explicit, because Solid resources have no invalidation registry; the key helper exists for apps that add one.
