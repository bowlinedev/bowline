# @bowlinedev/client

## 1.4.1

- Released with bowline 1.4.1; see the root CHANGELOG for what changed.

## 1.4.0

- Released with bowline 1.4.0; see the root CHANGELOG for what changed.

## 1.3.0

- Released with bowline 1.3.0; see the root CHANGELOG for what changed.

## 1.2.0

- Released with bowline 1.2.0; see the root CHANGELOG for what changed.

## 1.1.0

- Released with bowline 1.1.0; see the root CHANGELOG for what changed.

## 1.0.0

- Released with bowline 1.0.0; see the root CHANGELOG for what changed.

## 0.7.0

- Released with bowline 0.7.0; see the root CHANGELOG for what changed.

## 0.6.0

### Patch Changes

- Released with bowline 0.6.0.

## 0.5.0

### Minor Changes

- `procedureKey`, `walkProcedures`, `InputOf`, and `OutputOf` are shared with every binding, and `@bowlinedev/client/server` exports `createServerClient` with header forwarding and `fetch` cache passthrough.

## 0.4.0

### Minor Changes

- The `record` option delivers every call to a `RecordSink`, and `@bowlinedev/client/node` exports `fileSink` to write consumer contract files; `ContractField` carries `example`.

## 0.3.0

### Minor Changes

- `ContractDocument` and its companion types describe a parsed `bowline.contract.json`, including tool exposure and embedded schemas.

## 0.2.0

### Minor Changes

- Subscriptions as async iterables over server-sent events or WebSocket, typed uploads, `.safe` results with error variant narrowing, and the `idempotencyKey` call option.

## 0.1.0

### Minor Changes

- First public alpha: typed client runtime with Date and bigint hydration, BowlineError, and TanStack Query bindings.
