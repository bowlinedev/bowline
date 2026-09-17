# SvelteKit

A SvelteKit page loads on the server through `serverClient`, which forwards `event.fetch` and the caller's `cookie` and `authorization` headers. It updates in the browser through `@bowlinedev/svelte` stores.

source: examples/sveltekit/src/routes/+page.server.ts:1-10

```ts
import { serverClient } from "@bowlinedev/svelte";
import { createClient } from "$lib/bowline.js";
import type { PageServerLoad } from "./$types.js";

export const load: PageServerLoad = async (event) => {
  const client = serverClient(event, createClient, { url: "http://localhost:8080/api" });
  const page = await client.invoices.list({ limit: 20 });
  const health = await client.health();
  return { invoices: page.items, version: health.version };
};
```

In the browser, the page uses the stores from `svelte.md` against a client at `/api`. Vite proxies that to the Go server in dev and preview.

Tests: `cd examples/sveltekit && pnpm build && pnpm test:e2e` checks that the initial HTML already lists the seeded invoices, that a create adds a row without a reload, and that validation issues render.

Gotchas: `event.fetch` is what makes server loads work behind SvelteKit's request handling and lets it inline the result for hydration. Passing the global `fetch` loses both. Header forwarding is limited to `cookie` and `authorization` on purpose, so that a load never leaks a browser's other headers to the API. In production, use adapter-node's server or a reverse proxy for `/api`, because the Vite proxy only exists in dev and preview.
