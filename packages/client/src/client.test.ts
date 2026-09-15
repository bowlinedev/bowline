import { expect, expectTypeOf, test, vi } from "vitest";
import { createClient } from "./client.js";
import type { TypedError, UntypedError } from "./error.js";
import { BowlineError } from "./error.js";
import type { ContractRuntime, Mutation, Query, Subscription } from "./types.js";

const contract: ContractRuntime = {
  version: "0.1",
  hydrators: { User: [{ path: ["createdAt"], kind: "timestamp" }] },
  procedures: {
    "users.get": { kind: "query", method: "GET", output: "User" },
    "users.search": { kind: "query", method: "POST", output: "User" },
    "users.create": { kind: "mutation", method: "POST", output: "User" },
    health: { kind: "query", method: "GET" },
  },
};

interface Client {
  users: {
    get: Query<{ id: number }, { id: number; createdAt: Date }>;
    search: Query<{ q: string }, { id: number; createdAt: Date }>;
    create: Mutation<{ name: string; big: bigint }, { id: number; createdAt: Date }>;
  };
  health: Query<Record<string, never>, { ok: boolean }>;
}

function fakeFetch(
  handler: (url: string, init: RequestInit) => Response | Promise<Response>,
): typeof fetch {
  return vi.fn(async (input: string | URL | Request, init?: RequestInit) =>
    handler(String(input), init ?? {}),
  ) as unknown as typeof fetch;
}

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

test("GET queries put input in the query string and hydrate the output", async () => {
  const calls: { url: string; init: RequestInit }[] = [];
  const client = createClient(contract, {
    url: "http://api.test/api",
    fetch: fakeFetch((url, init) => {
      calls.push({ url, init });
      return json({ id: 1, createdAt: "2026-09-15T12:00:00Z" });
    }),
  }) as Client;
  const user = await client.users.get({ id: 1 });
  expect(user.createdAt).toBeInstanceOf(Date);
  expect(calls[0]?.url).toBe("http://api.test/api/users.get?input=%7B%22id%22%3A1%7D");
  expect(calls[0]?.init.method).toBe("GET");
  expect(client.users.get.kind).toBe("query");
});

test("POST calls send a JSON body with bigint support", async () => {
  const calls: { url: string; init: RequestInit }[] = [];
  const client = createClient(contract, {
    url: "http://api.test/api/",
    fetch: fakeFetch((url, init) => {
      calls.push({ url, init });
      return json({ id: 2, createdAt: "2026-09-15T12:00:00Z" });
    }),
  }) as Client;
  await client.users.create({ name: "ada", big: 9007199254740993n });
  expect(calls[0]?.url).toBe("http://api.test/api/users.create");
  expect(calls[0]?.init.method).toBe("POST");
  expect(calls[0]?.init.body).toBe('{"name":"ada","big":"9007199254740993"}');
  expect(new Headers(calls[0]?.init.headers).get("content-type")).toBe("application/json");
  await client.users.search({ q: "x" });
  expect(calls[1]?.init.method).toBe("POST");
  expect(client.users.create.kind).toBe("mutation");
});

test("empty inputs may be omitted", async () => {
  const calls: string[] = [];
  const client = createClient(contract, {
    url: "http://api.test",
    fetch: fakeFetch((url) => {
      calls.push(url);
      return json({ ok: true });
    }),
  }) as Client;
  expect(await client.health()).toEqual({ ok: true });
  expect(calls[0]).toBe("http://api.test/health");
});

test("error envelopes become BowlineError", async () => {
  const client = createClient(contract, {
    url: "http://api.test",
    fetch: fakeFetch(() =>
      json(
        {
          error: {
            code: "INVALID_ARGUMENT",
            message: "invalid input",
            issues: [{ path: ["id"], rule: "required", message: "is required" }],
          },
        },
        400,
      ),
    ),
  }) as Client;
  const err = await client.users.get({ id: 0 }).catch((e: unknown) => e);
  expect(err).toBeInstanceOf(BowlineError);
  const be = err as BowlineError;
  expect(be.code).toBe("INVALID_ARGUMENT");
  expect(be.status).toBe(400);
  expect(be.issues[0]?.rule).toBe("required");
});

test("non-envelope failures and network errors map to codes", async () => {
  const html = createClient(contract, {
    url: "http://api.test",
    fetch: fakeFetch(() => new Response("<html>bad gateway</html>", { status: 502 })),
  }) as Client;
  const e1 = (await html.users.get({ id: 1 }).catch((e: unknown) => e)) as BowlineError;
  expect(e1.code).toBe("UNKNOWN");
  expect(e1.status).toBe(502);
  const down = createClient(contract, {
    url: "http://api.test",
    fetch: fakeFetch(() => {
      throw new TypeError("fetch failed");
    }),
  }) as Client;
  const e2 = (await down.users.get({ id: 1 }).catch((e: unknown) => e)) as BowlineError;
  expect(e2.code).toBe("UNAVAILABLE");
  expect(e2.status).toBe(0);
});

test("headers come from static values, functions, and per-call options", async () => {
  const seen: Headers[] = [];
  const client = createClient(contract, {
    url: "http://api.test",
    headers: async () => ({ authorization: "Bearer t" }),
    fetch: fakeFetch((_url, init) => {
      seen.push(new Headers(init.headers));
      return json({ id: 1, createdAt: "2026-09-15T12:00:00Z" });
    }),
  }) as Client;
  await client.users.get({ id: 1 }, { headers: { "x-trace": "abc" } });
  expect(seen[0]?.get("authorization")).toBe("Bearer t");
  expect(seen[0]?.get("x-trace")).toBe("abc");
  expect(seen[0]?.get("accept")).toBe("application/json");
});

test("abort rejects with CANCELED", async () => {
  const controller = new AbortController();
  const client = createClient(contract, {
    url: "http://api.test",
    fetch: fakeFetch((_url, init) => {
      controller.abort();
      const reason = new DOMException("aborted", "AbortError");
      if (init.signal?.aborted) {
        throw reason;
      }
      return json({});
    }),
  }) as Client;
  const err = (await client.users
    .get({ id: 1 }, { signal: controller.signal })
    .catch((e: unknown) => e)) as BowlineError;
  expect(err.code).toBe("CANCELED");
});

test("safe returns a discriminated result and hydrates typed details", async () => {
  const typedContract: ContractRuntime = {
    version: "1.0",
    hydrators: { InvoiceLocked: [{ path: ["since"], kind: "timestamp" }] },
    procedures: {
      "invoices.void": { kind: "mutation", method: "POST", errors: ["InvoiceLocked"] },
    },
  };
  interface Typed {
    invoices: {
      void: Mutation<
        { id: number },
        { id: number },
        TypedError<"InvoiceLocked", { id: number; since: Date }> | UntypedError
      >;
    };
  }
  const client = createClient(typedContract, {
    url: "http://api.test",
    fetch: fakeFetch(() =>
      json(
        {
          error: {
            code: "FAILED_PRECONDITION",
            message: "locked",
            type: "InvoiceLocked",
            details: { id: 4, since: "2026-09-15T00:00:00Z" },
          },
        },
        412,
      ),
    ),
  }) as Typed;
  const result = await client.invoices.void.safe({ id: 4 });
  expect(result.ok).toBe(false);
  if (!result.ok) {
    expect(result.error.type).toBe("InvoiceLocked");
    if (result.error.type === "InvoiceLocked") {
      expect(result.error.details.since).toBeInstanceOf(Date);
      expectTypeOf(result.error.details.id).toEqualTypeOf<number>();
    }
  }
  const okClient = createClient(typedContract, {
    url: "http://api.test",
    fetch: fakeFetch(() => json({ id: 4 })),
  }) as Typed;
  const ok = await okClient.invoices.void.safe({ id: 4 });
  expect(ok).toEqual({ ok: true, value: { id: 4 } });
});

const sseContract: ContractRuntime = {
  version: "1.0",
  hydrators: { Change: [{ path: ["at"], kind: "timestamp" }] },
  procedures: { "invoices.watch": { kind: "subscription", method: "GET", output: "Change" } },
};

interface WatchClient {
  invoices: { watch: Subscription<{ limit: number }, { id: number; at: Date }> };
}

function sseResponse(text: string): Response {
  const encoder = new TextEncoder();
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(encoder.encode(text));
      controller.close();
    },
  });
  return new Response(stream, { status: 200, headers: { "content-type": "text/event-stream" } });
}

test("subscriptions iterate hydrated messages until done", async () => {
  const seen: RequestInit[] = [];
  const client = createClient(sseContract, {
    url: "http://api.test",
    fetch: fakeFetch((_url, init) => {
      seen.push(init);
      return sseResponse(
        'event: message\ndata: {"id":1,"at":"2026-09-15T00:00:00Z"}\n\nevent: message\ndata: {"id":2,"at":"2026-09-16T00:00:00Z"}\n\nevent: done\ndata: {}\n\n',
      );
    }),
  }) as WatchClient;
  const values = [];
  for await (const v of client.invoices.watch({ limit: 5 })) {
    values.push(v);
  }
  expect(values.map((v) => v.id)).toEqual([1, 2]);
  expect(values[0]?.at).toBeInstanceOf(Date);
  expect(new Headers(seen[0]?.headers).get("accept")).toBe("text/event-stream");
  expect(client.invoices.watch.kind).toBe("subscription");
});

test("subscription error events reject with BowlineError", async () => {
  const client = createClient(sseContract, {
    url: "http://api.test",
    fetch: fakeFetch(() =>
      sseResponse(
        'event: message\ndata: {"id":1,"at":"2026-09-15T00:00:00Z"}\n\nevent: error\ndata: {"error":{"code":"NOT_FOUND","message":"gone"}}\n\n',
      ),
    ),
  }) as WatchClient;
  const values: number[] = [];
  const err = await (async () => {
    try {
      for await (const v of client.invoices.watch({ limit: 1 })) {
        values.push(v.id);
      }
    } catch (e) {
      return e;
    }
    return undefined;
  })();
  expect(values).toEqual([1]);
  expect(err).toBeInstanceOf(BowlineError);
  expect((err as BowlineError).code).toBe("NOT_FOUND");
});

test("subscribe delivers callbacks and unsubscribe aborts the request", async () => {
  let aborted = false;
  const client = createClient(sseContract, {
    url: "http://api.test",
    fetch: fakeFetch((_url, init) => {
      init.signal?.addEventListener("abort", () => {
        aborted = true;
      });
      const encoder = new TextEncoder();
      const stream = new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(
            encoder.encode('event: message\ndata: {"id":7,"at":"2026-09-15T00:00:00Z"}\n\n'),
          );
        },
      });
      return new Response(stream, { status: 200 });
    }),
  }) as WatchClient;
  const got: number[] = [];
  let done = false;
  const stop = client.invoices.watch.subscribe(
    { limit: 1 },
    {
      onData: (v) => got.push(v.id),
      onDone: () => {
        done = true;
      },
    },
  );
  await new Promise((r) => setTimeout(r, 20));
  expect(got).toEqual([7]);
  stop();
  await new Promise((r) => setTimeout(r, 20));
  expect(aborted).toBe(true);
  expect(done).toBe(false);
});
