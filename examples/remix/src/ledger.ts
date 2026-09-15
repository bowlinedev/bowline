import { createServerClient } from "@bowline/client/server";
import { type Client, createClient } from "./bowline.js";

export const backend = `${process.env.LEDGER_URL ?? "http://localhost:8080"}/api`;

export function ledger(request: Request): Client {
  return createServerClient(createClient, { url: backend, request });
}
