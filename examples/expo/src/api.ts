import { bowlineQuery } from "@bowline/react-query";
import Constants from "expo-constants";
import { createClient } from "./bowline";

const extra = (Constants.expoConfig?.extra ?? {}) as { apiUrl?: string };

export const apiUrl = extra.apiUrl ?? "http://localhost:8080/api";

export const client = createClient({
  url: apiUrl,
  fetch: (input, init) => globalThis.fetch(input, init),
});

export const bq = bowlineQuery(client);
