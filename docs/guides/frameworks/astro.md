# Astro

An Astro page calls the API at request time through `createServerClient`. A Solid island keeps the page live through `@bowlinedev/solid`.

source: examples/astro/src/pages/index.astro:2-10

```ts
import { createServerClient } from "@bowlinedev/client/server";
import { createClient } from "../bowline.js";
import Invoices from "../components/Invoices.tsx";

const ledger = createServerClient(createClient, {
  url: "http://localhost:8080/api",
  request: Astro.request,
});
const page = await ledger.invoices.list({ limit: 20 });
```

The island in `examples/astro/src/components/Invoices.tsx` is shown in `solid.md`. Browser calls go to `/api`, which `src/pages/api/[...path].ts` proxies to the Go server, so the app stays same-origin without needing CORS.

Tests: `cd examples/astro && pnpm build && pnpm test:e2e` checks the server-rendered rows, the island's hydration, a create through the island, and validation issues.

Gotchas: mount the island with `client:only`. A `client:load` island renders on the server, and its resource would fetch a relative URL there. The server-rendered table above it already handles first paint. Astro's node adapter has no dev proxy, which is why the API route exists.
