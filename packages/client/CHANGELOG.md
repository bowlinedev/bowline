# @bowline/client

## 0.4.0

### Minor Changes

- The `record` option delivers every call to a `RecordSink`, and `@bowline/client/node` exports `fileSink` to write consumer contract files; `ContractField` carries `example`.

## 0.3.0

### Minor Changes

- `ContractDocument` and its companion types describe a parsed `bowline.contract.json`, including tool exposure and embedded schemas.

## 0.2.0

### Minor Changes

- Subscriptions as async iterables over server-sent events or WebSocket, typed uploads, `.safe` results with error variant narrowing, and the `idempotencyKey` call option.

## 0.1.0

### Minor Changes

- First public alpha: typed client runtime with Date and bigint hydration, BowlineError, and TanStack Query bindings.
