---
"@bowline/client": minor
"@bowline/react-query": patch
---

`@bowline/client` shares `procedureKey`, `walkProcedures`, `InputOf`, and `OutputOf` with every binding, gains the `record` option with `@bowline/client/node`'s `fileSink`, and exports `createServerClient` from `@bowline/client/server`; `@bowline/react-query` builds on the shared helpers.
