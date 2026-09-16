import { expect, test, vi } from "vitest";
import { createClient } from "./client.js";
import { BowlineError } from "./error.js";
import type { ContractRuntime, Query } from "./types.js";

const contract: ContractRuntime = {
  version: "0.1",
  hydrators: { User: [{ path: ["createdAt"], kind: "timestamp" }] },
  procedures: {
    "users.get": { kind: "query", method: "GET", output: "User" },
    health: { kind: "query", method: "GET" },
  },
};

interface Client {
  users: { get: Query<{ id: number }, { id: number; createdAt: Date }> };
  health: Query<Record<string, never>, { ok: boolean }>;
}

function rng(seed: number): () => number {
  let state = seed >>> 0 || 0x9e3779b9;
  return () => {
    state ^= state << 13;
    state ^= state >>> 17;
    state ^= state << 5;
    state >>>= 0;
    return state / 0x100000000;
  };
}

const statuses = [200, 201, 204, 400, 401, 403, 404, 408, 409, 412, 429, 500, 501, 503];

const bodies = [
  "",
  "null",
  "{}",
  "[]",
  "not json at all",
  '{"error":{}}',
  '{"error":null}',
  '{"error":{"code":"NOT_FOUND"}}',
  '{"error":{"code":"NOT_FOUND","message":"gone"}}',
  '{"error":{"code":123,"message":null}}',
  '{"error":{"code":"INVALID_ARGUMENT","message":"bad","issues":[{"path":"id"}]}}',
  '{"error":{"code":"INVALID_ARGUMENT","message":"bad","issues":"not an array"}}',
  '{"id":1,"createdAt":"2026-01-01T00:00:00Z"}',
  '{"id":1,"createdAt":"not a date"}',
  '{"id":1,"createdAt":null}',
  '{"id":1}',
  '{"createdAt":{"nested":"2026-01-01T00:00:00Z"}}',
  '"a bare string"',
  "12345",
  "true",
  '{"__proto__":{"polluted":true}}',
  "[[[[[[[[[[1]]]]]]]]]]",
  '{"error":{"code":"NOT_FOUND","message":"gone","details":{"a":[1,2,{"b":null}]}}}',
];

const contentTypes = [
  "application/json",
  "application/json; charset=utf-8",
  "text/plain",
  "text/html",
  "",
  "application/octet-stream",
];

function pick<T>(items: readonly T[], r: number): T {
  return items[Math.floor(r * items.length) % items.length] as T;
}

function responseFor(seed: number): { response: Response; status: number; body: string } {
  const next = rng(seed);
  const status = pick(statuses, next());
  const body = pick(bodies, next());
  const contentType = pick(contentTypes, next());
  const headers: Record<string, string> = {};
  if (contentType !== "") headers["content-type"] = contentType;
  const init: ResponseInit = { status, headers };
  const response = status === 204 ? new Response(null, init) : new Response(body, init);
  return { response, status, body };
}

function clientFor(response: Response): Client {
  return createClient(contract, {
    url: "http://api.test/api",
    fetch: vi.fn(async () => response.clone()) as unknown as typeof fetch,
  }) as Client;
}

test("every rejection is a BowlineError, for any status, body, and content type", async () => {
  for (let seed = 1; seed <= 2000; seed++) {
    const { response, status, body } = responseFor(seed);
    const client = clientFor(response);
    try {
      await client.users.get({ id: 1 });
    } catch (thrown) {
      if (!(thrown instanceof BowlineError)) {
        throw new Error(
          `seed ${seed} (status ${status}, body ${JSON.stringify(body)}) threw ${String(thrown)}`,
        );
      }
      expect(typeof thrown.code).toBe("string");
      expect(thrown.code.length).toBeGreaterThan(0);
      expect(typeof thrown.message).toBe("string");
    }
  }
});

test("a safe call never rejects and always reports one of ok or error", async () => {
  for (let seed = 1; seed <= 2000; seed++) {
    const { response, status, body } = responseFor(seed);
    const client = clientFor(response);
    const result = await client.users.get.safe({ id: 1 });
    if (result.ok === false) {
      if (!(result.error instanceof BowlineError)) {
        throw new Error(
          `seed ${seed} (status ${status}, body ${JSON.stringify(body)}) reported a non-BowlineError`,
        );
      }
      expect(typeof result.error.code).toBe("string");
    } else {
      expect(result.ok).toBe(true);
    }
  }
});

test("hydration never pollutes Object.prototype and never throws a bare error", async () => {
  for (let seed = 1; seed <= 2000; seed++) {
    const { response, status, body } = responseFor(seed);
    const client = clientFor(response);
    const result = await client.users.get.safe({ id: 1 });
    if (result.ok === false && !(result.error instanceof BowlineError)) {
      throw new Error(
        `seed ${seed} (status ${status}, body ${JSON.stringify(body)}) reported a non-BowlineError`,
      );
    }
  }
  expect(({} as Record<string, unknown>).polluted).toBeUndefined();
  expect(Object.hasOwn(Object.prototype, "polluted")).toBe(false);
});
