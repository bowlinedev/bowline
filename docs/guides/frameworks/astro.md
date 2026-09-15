# Astro

An Astro page calls the API at request time through `createServerClient`, and a Solid island keeps the page live through `@bowline/solid`.

source: examples/astro/src/pages/index.astro:2-10

```ts
import { createServerClient } from "@bowline/client/server";
import { createClient } from "../bowline.js";
import Invoices from "../components/Invoices.tsx";

const ledger = createServerClient(createClient, {
  url: "http://localhost:8080/api",
  request: Astro.request,
});
const page = await ledger.invoices.list({ limit: 20 });
```

The island in `examples/astro/src/components/Invoices.tsx` is shown in `solid.md`. Browser calls go to `/api`, which `src/pages/api/[...path].ts` proxies to the Go server so the app stays same-origin without CORS.

Proof: `cd examples/astro && pnpm build && pnpm test:e2e` asserts the server-rendered rows, the island's hydration, a create through the island, and validation issues.

Gotchas: mount the island `client:only`, because a `client:load` island renders on the server and its resource fetches a relative URL there; the server-rendered table above it already covers first paint. Astro's node adapter has no dev proxy, hence the API route.
