# @bowline/vue

Vue bindings for [Bowline](https://github.com/bowlinedev/bowline).

Bowline turns your Go code into typed API clients. This package connects that client to Vue.

## Install

```bash
npm install @bowline/vue @bowline/client
```

## Use

```ts
import { bowlineVue } from "@bowline/vue";
import { createClient } from "./bowline";

const bq = bowlineVue(createClient({ url: "/api" }));
```

Cache keys are built from the procedure path, so every Bowline binding uses the same key for the same call.

Docs: [docs/guides/frameworks/vue.md](https://github.com/bowlinedev/bowline/blob/main/docs/guides/frameworks/vue.md)

Apache-2.0
