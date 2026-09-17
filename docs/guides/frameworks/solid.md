# Solid

`@bowlinedev/solid` wraps every query in `createResource` and every mutation in an action with a pending signal and an error signal.

source: examples/astro/src/components/Invoices.tsx:5-9

```tsx
const ledger = bowlineSolid(client);

export default function Invoices() {
  const [list, { refetch }] = ledger.invoices.list.resource({ limit: 20 });
  const [create, state] = ledger.invoices.create.action();
```

`resource(input)` accepts a plain input or an accessor. With an accessor, the resource refetches whenever the input changes. `action()` returns `[mutate, state]`. `mutate(input)` resolves with the output, and `state()` has `pending` and a `BowlineError` if the call failed.

Tests: `pnpm --filter @bowlinedev/solid test`. The Astro example's island under `examples/astro` uses both helpers, and its Playwright suite drives them. See `astro.md`.

Gotchas: a resource created during server rendering runs its fetcher on the server, where a relative URL like `/api` has no origin. Render the island client-only, or give the server an absolute URL. `refetch()` after a mutation is explicit, because Solid resources have no invalidation registry. The key helper exists for apps that add one.
