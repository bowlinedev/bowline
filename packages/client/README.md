# @bowlinedev/client

The client runtime for [Bowline](https://github.com/bowlinedev/bowline).

Bowline turns your Go code into typed API clients. This package is what the generated client uses to make calls.

## Install

```bash
npm install @bowlinedev/client
```

## Use

```ts
import { createClient } from "./bowline";

const client = createClient({ url: "http://localhost:8080/api" });
const invoice = await client.invoices.get({ id: 3 });
```

You do not write the types. `bowline gen` writes them from your Go code.

## Entry points

- `@bowlinedev/client`: in the browser or on the server
- `@bowlinedev/client/server`: for server-side calls, forwards headers
- `@bowlinedev/client/node`: writes a record of what your client uses

Docs: [bowlinedev/bowline](https://github.com/bowlinedev/bowline)

Apache-2.0
