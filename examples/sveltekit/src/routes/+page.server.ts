import { serverClient } from "@bowline/svelte";
import { createClient } from "$lib/bowline.js";
import type { PageServerLoad } from "./$types.js";

export const load: PageServerLoad = async (event) => {
  const client = serverClient(event, createClient, { url: "http://localhost:8080/api" });
  const page = await client.invoices.list({ limit: 20 });
  const health = await client.health();
  return { invoices: page.items, version: health.version };
};
