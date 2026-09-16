# @bowlinedev/solid

Solid bindings for [Bowline](https://github.com/bowlinedev/bowline).

Bowline turns your Go code into typed API clients. This package connects that client to Solid.

## Install

```bash
npm install @bowlinedev/solid @bowlinedev/client
```

## Use

```ts
import { bowlineSolid } from "@bowlinedev/solid";
import { createClient } from "./bowline";

const bq = bowlineSolid(createClient({ url: "/api" }));
```

Cache keys are built from the procedure path, so every Bowline binding uses the same key for the same call.

Docs: [docs/guides/frameworks/solid.md](https://github.com/bowlinedev/bowline/blob/main/docs/guides/frameworks/solid.md)

Apache-2.0
