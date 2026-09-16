import { bowlineStores } from "@bowlinedev/svelte";
import { createClient } from "./bowline.js";

export const client = createClient({ url: "/api" });
export const stores = bowlineStores(client);
