# @bowlinedev/svelte

Svelte bindings for [Bowline](https://github.com/bowlinedev/bowline).

Bowline turns your Go code into typed API clients. This package connects that client to Svelte.

## Install

```bash
npm install @bowlinedev/svelte @bowlinedev/client
```

## Use

```ts
import { bowlineStores } from "@bowlinedev/svelte";
import { createClient } from "./bowline";

const bq = bowlineStores(createClient({ url: "/api" }));
```

Cache keys are built from the procedure path, so every Bowline binding uses the same key for the same call.

Docs: [docs/guides/frameworks/svelte.md](https://github.com/bowlinedev/bowline/blob/main/docs/guides/frameworks/svelte.md)

Apache-2.0
