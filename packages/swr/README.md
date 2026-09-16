# @bowline/swr

SWR bindings for [Bowline](https://github.com/bowlinedev/bowline).

Bowline turns your Go code into typed API clients. This package connects that client to SWR.

## Install

```bash
npm install @bowline/swr @bowline/client
```

## Use

```ts
import { bowlineSWR } from "@bowline/swr";
import { createClient } from "./bowline";

const bq = bowlineSWR(createClient({ url: "/api" }));
```

Cache keys are built from the procedure path, so every Bowline binding uses the same key for the same call.

Docs: [docs/guides/frameworks/swr.md](https://github.com/bowlinedev/bowline/blob/main/docs/guides/frameworks/swr.md)

Apache-2.0
