# @bowlinedev/react-query

TanStack Query bindings for [Bowline](https://github.com/bowlinedev/bowline).

Bowline turns your Go code into typed API clients. This package connects that client to TanStack Query.

## Install

```bash
npm install @bowlinedev/react-query @bowlinedev/client
```

## Use

```ts
import { bowlineQuery } from "@bowlinedev/react-query";
import { createClient } from "./bowline";

const bq = bowlineQuery(createClient({ url: "/api" }));
```

Cache keys are built from the procedure path, so every Bowline binding uses the same key for the same call.

Docs: [docs/guides/frameworks/react-query.md](https://github.com/bowlinedev/bowline/blob/main/docs/guides/frameworks/react-query.md)

Apache-2.0
