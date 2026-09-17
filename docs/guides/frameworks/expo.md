# Expo

The generated client uses plain `fetch`, so a React Native app can use it unchanged. The only mobile-specific part is where the API URL comes from.

source: examples/expo/src/api.ts:5-14

```ts
const extra = (Constants.expoConfig?.extra ?? {}) as { apiUrl?: string };

export const apiUrl = extra.apiUrl ?? "http://localhost:8080/api";

export const client = createClient({
  url: apiUrl,
  fetch: (input, init) => globalThis.fetch(input, init),
});

export const bq = bowlineQuery(client);
```

source: examples/expo/app/index.tsx:6-7

```tsx
export default function InvoicesScreen() {
  const list = useQuery(bq.invoices.list.queryOptions({ limit: 20 }));
```

`extra.apiUrl` in `app.json` is where each build points. An Android emulator reaches the host at `10.0.2.2`, an iOS simulator at `localhost`, and a physical device at the machine's LAN address. Screens use `@bowlinedev/react-query` the same way as on the web. The create screen renders `BowlineError.issues` from a failed mutation.

Tests: `cd examples/expo && pnpm test` runs the Jest suite with a mocked `fetch`, checking the list and the validation issues, on every push. `maestro/flow.yaml` drives the real app on an Android emulator through the `expo-device` workflow. That runs weekly and on demand, since an emulator job takes too long for every pull request.

Gotchas: resolve `fetch` at call time, as shown above, so that tests can replace `globalThis.fetch` after the module has loaded. Metro does not read `exports` maps the way bundlers do. The example's Jest config maps the workspace packages to their sources, while a published app resolves them from `node_modules` normally. Timestamps arrive as `Date` objects through the client's hydration, same as in a browser.
