import { readFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";
import { createClient } from "./client.js";
import { createServerClient } from "./server.js";
import type { ContractRuntime, Query } from "./types.js";

const contract: ContractRuntime = {
  version: "1.4",
  hydrators: {},
  procedures: { "users.get": { kind: "query", method: "GET" } },
};

type Client = { users: { get: Query<{ id: number }, { id: number }> } };

function capture(): { fetch: typeof fetch; calls: { url: string; init: RequestInit }[] } {
  const calls: { url: string; init: RequestInit }[] = [];
  const fetchFn = (async (input: string | URL | Request, init?: RequestInit) => {
    calls.push({ url: String(input), init: init ?? {} });
    return new Response(`{"id":1}`, { status: 200 });
  }) as typeof fetch;
  return { fetch: fetchFn, calls };
}

describe("createServerClient", () => {
  it("forwards only the listed headers from a Request", async () => {
    const { fetch, calls } = capture();
    const request = new Request("http://app/page", {
      headers: { cookie: "session=1", authorization: "Bearer t", "x-forwarded-for": "1.1.1.1" },
    });
    const client = createServerClient((options) => createClient(contract, options) as Client, {
      url: "http://api/api",
      request,
      fetch,
    });
    await client.users.get({ id: 1 });
    const headers = new Headers(calls[0]?.init.headers);
    expect(headers.get("cookie")).toBe("session=1");
    expect(headers.get("authorization")).toBe("Bearer t");
    expect(headers.get("x-forwarded-for")).toBeNull();
  });

  it("reads Node-style header objects and custom lists", async () => {
    const { fetch, calls } = capture();
    const client = createServerClient((options) => createClient(contract, options) as Client, {
      url: "http://api/api",
      request: { headers: { Cookie: "a=1", "X-Tenant": ["t1", "t2"], authorization: undefined } },
      forwardHeaders: ["x-tenant"],
      headers: { "x-static": "yes" },
      fetch,
    });
    await client.users.get({ id: 1 });
    const headers = new Headers(calls[0]?.init.headers);
    expect(headers.get("x-tenant")).toBe("t1, t2");
    expect(headers.get("cookie")).toBeNull();
    expect(headers.get("x-static")).toBe("yes");
  });

  it("passes cache and next through to fetch", async () => {
    const { fetch, calls } = capture();
    const client = createServerClient((options) => createClient(contract, options) as Client, {
      url: "http://api/api",
      fetch,
      cache: "no-store",
      next: { revalidate: 60, tags: ["users"] },
    });
    await client.users.get({ id: 1 });
    const init = calls[0]?.init as RequestInit & { next?: unknown };
    expect(init.cache).toBe("no-store");
    expect(init.next).toEqual({ revalidate: 60, tags: ["users"] });
  });

  it("stays out of the browser entry", async () => {
    const source = await readFile(new URL("./index.ts", import.meta.url), "utf8");
    expect(source).not.toContain("server.js");
  });
});
