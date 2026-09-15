import { createClient } from "./bowline.js";

export const apiUrl = import.meta.env.PUBLIC_API_URL ?? "/api";

export const client = createClient({ url: apiUrl });
