import { createServerClient, type ServerClientOptions } from "@bowlinedev/client/server";
import { headers } from "next/headers";
import { type Client, createClient } from "./bowline";

export const backend = `${process.env.LEDGER_URL ?? "http://localhost:8080"}/api`;

export async function ledger(options: Partial<ServerClientOptions> = {}): Promise<Client> {
  const incoming = await headers();
  return createServerClient(createClient, {
    url: backend,
    request: { headers: incoming },
    ...options,
  });
}
