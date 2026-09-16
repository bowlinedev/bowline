# Ledger on Expo

An Expo Router app that lists and creates invoices through the ledger's generated client, `@bowlinedev/client`, and `@bowlinedev/react-query`.

```bash
pnpm --filter ledger-expo gen
pnpm --filter ledger-expo test
pnpm --filter ledger-expo start
```

The API base URL comes from `extra.apiUrl` in `app.json`; point it at a reachable ledger server (an emulator needs the host's address, not `localhost`). `maestro/flow.yaml` drives a device build against the seeded ledger and runs from the `expo-device` workflow on demand and weekly; the Jest suite in `__tests__` runs on every push with a mocked `fetch`.
