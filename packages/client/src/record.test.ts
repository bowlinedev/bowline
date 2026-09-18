import { mkdtemp, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { createClient } from "./client.js";
import { fileSink } from "./node.js";
import { canonicalInput, type Interaction } from "./record.js";
import type { ContractRuntime, Mutation, Query } from "./types.js";

const contract: ContractRuntime = {
  version: "1.4",
  hydrators: { Invoice: [{ path: ["createdAt"], kind: "timestamp" }] },
  procedures: {
    "invoices.get": { kind: "query", method: "GET", output: "Invoice" },
    "invoices.create": { kind: "mutation", method: "POST", output: "Invoice" },
  },
};

type Client = {
  invoices: {
    get: Query<{ id: number }, { id: number; createdAt: Date }>;
    create: Mutation<{ note?: string }, { id: number; createdAt: Date }>;
  };
};

function fakeFetch(status: number, body: string): typeof fetch {
  return (async () => new Response(body, { status })) as typeof fetch;
}

describe("record", () => {
  it("delivers every response to the sink before hydration", async () => {
    const seen: Interaction[] = [];
    const sink = { consumer: "test", write: (i: Interaction) => void seen.push(i) };
    const ok = createClient(contract, {
      url: "http://x/api",
      fetch: fakeFetch(200, `{"id":3,"createdAt":"2026-01-01T00:00:00Z"}`),
      record: sink,
    }) as Client;
    const value = await ok.invoices.get({ id: 3 });
    expect(value.createdAt).toBeInstanceOf(Date);
    const failing = createClient(contract, {
      url: "http://x/api",
      fetch: fakeFetch(404, `{"error":{"code":"NOT_FOUND","message":"nope"}}`),
      record: sink,
    }) as Client;
    await expect(failing.invoices.get({ id: 9 })).rejects.toThrow("nope");
    expect(seen).toEqual([
      {
        procedure: "invoices.get",
        method: "GET",
        input: { id: 3 },
        response: { status: 200, body: { id: 3, createdAt: "2026-01-01T00:00:00Z" } },
      },
      {
        procedure: "invoices.get",
        method: "GET",
        input: { id: 9 },
        response: { status: 404, body: { error: { code: "NOT_FOUND", message: "nope" } } },
      },
    ]);
  });

  it("canonicalizes input with sorted keys and no undefined", () => {
    expect(canonicalInput({ b: 1, a: [2, { d: undefined, c: 3n }] })).toBe(
      '{"a":[2,{"c":"3"}],"b":1}',
    );
    expect(canonicalInput(undefined)).toBe("{}");
  });

  it("writes a deduplicated consumer file", async () => {
    const dir = await mkdtemp(join(tmpdir(), "bowline-record-"));
    const path = join(dir, "contracts", "consumers", "ledger-web.json");
    const sink = fileSink("ledger-web", path, { provider: "ledger" });
    const client = createClient(contract, {
      url: "http://x/api",
      fetch: fakeFetch(200, `{"id":1,"createdAt":"2026-01-01T00:00:00Z"}`),
      record: sink,
    }) as Client;
    await client.invoices.create({ note: "a" });
    await client.invoices.create({ note: "a" });
    await client.invoices.get({ id: 1 });
    await sink.flush();
    const document = JSON.parse(await readFile(path, "utf8")) as {
      bowline: string;
      consumer: string;
      provider: string;
      interactions: Interaction[];
    };
    expect(document.bowline).toBe("1.4");
    expect(document.consumer).toBe("ledger-web");
    expect(document.provider).toBe("ledger");
    expect(document.interactions.map((i) => i.procedure)).toEqual([
      "invoices.create",
      "invoices.get",
    ]);
    expect(sink.interactions()).toHaveLength(2);
  });

  it("keeps Node imports out of the browser entry", async () => {
    for (const file of ["index.ts", "client.ts", "record.ts", "types.ts"]) {
      const source = await readFile(new URL(`./${file}`, import.meta.url), "utf8");
      expect(source, file).not.toContain("node:");
    }
  });
});
