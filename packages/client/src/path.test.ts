import { describe, expect, it } from "vitest";
import { resolvePath } from "./client.js";
import type { ProcedureRuntime } from "./types.js";

const query = (path?: string): ProcedureRuntime =>
  path === undefined ? { kind: "query", method: "GET" } : { kind: "query", method: "GET", path };
const mutation = (path?: string): ProcedureRuntime =>
  path === undefined
    ? { kind: "mutation", method: "POST" }
    : { kind: "mutation", method: "POST", path };

describe("resolvePath", () => {
  it("leaves a procedure without a custom path alone", () => {
    expect(resolvePath("invoices.list", query(), { limit: 20 })).toEqual({
      path: "invoices.list",
      payload: { limit: 20 },
    });
  });

  it("substitutes a parameter and drops it from the payload", () => {
    expect(resolvePath("invoices.get", query("invoices/{id}"), { id: 3 })).toEqual({
      path: "invoices/3",
      payload: undefined,
    });
  });

  it("keeps the fields the path did not consume", () => {
    expect(resolvePath("invoices.get", query("invoices/{id}"), { id: 3, expand: true })).toEqual({
      path: "invoices/3",
      payload: { expand: true },
    });
  });

  it("substitutes several parameters in order", () => {
    expect(
      resolvePath("invoices.line", query("invoices/{invoiceId}/lines/{lineId}"), {
        invoiceId: 3,
        lineId: "abc",
      }),
    ).toEqual({ path: "invoices/3/lines/abc", payload: undefined });
  });

  it("percent-encodes a value that would otherwise split the path", () => {
    expect(resolvePath("docs.get", query("docs/{slug}"), { slug: "a/b" })).toEqual({
      path: "docs/a%2Fb",
      payload: undefined,
    });
    expect(resolvePath("docs.get", query("docs/{slug}"), { slug: "caf é?" }).path).toBe(
      "docs/caf%20%C3%A9%3F",
    );
  });

  it("supports a literal path with no parameters", () => {
    expect(resolvePath("invoices.list", query("invoices"), { limit: 20 })).toEqual({
      path: "invoices",
      payload: { limit: 20 },
    });
  });

  it("works for a mutation body", () => {
    expect(resolvePath("invoices.void", mutation("invoices/{id}/void"), { id: 7 })).toEqual({
      path: "invoices/7/void",
      payload: undefined,
    });
  });

  it("throws when a parameter is missing", () => {
    expect(() => resolvePath("invoices.get", query("invoices/{id}"), {})).toThrow(
      /path parameter "id"/,
    );
    expect(() => resolvePath("invoices.get", query("invoices/{id}"), { id: null })).toThrow(
      /path parameter "id"/,
    );
  });

  it("stringifies a bigint parameter without losing precision", () => {
    expect(
      resolvePath("invoices.get", query("invoices/{id}"), { id: 9007199254740993n }).path,
    ).toBe("invoices/9007199254740993");
  });
});

import { createClient } from "./client.js";
import type { ContractRuntime, Mutation, Query } from "./types.js";

const restContract: ContractRuntime = {
  version: "0.1",
  hydrators: {},
  procedures: {
    "invoices.get": { kind: "query", method: "GET", path: "invoices/{id}" },
    "invoices.list": { kind: "query", method: "GET", path: "invoices" },
    "invoices.create": { kind: "mutation", method: "POST", path: "invoices" },
    "invoices.line": { kind: "query", method: "GET", path: "invoices/{invoiceId}/lines/{lineId}" },
  },
};

interface RestClient {
  invoices: {
    get: Query<{ id: number }, { id: number }>;
    list: Query<{ limit: number }, { id: number }>;
    create: Mutation<{ total: string }, { id: number }>;
    line: Query<{ invoiceId: number; lineId: string }, { id: number }>;
  };
}

function first(seen: { url: string; init: RequestInit }[]): { url: string; init: RequestInit } {
  const entry = seen[0];
  if (entry === undefined) {
    throw new Error("no request was sent");
  }
  return entry;
}

function restClient(seen: { url: string; init: RequestInit }[]): RestClient {
  return createClient(restContract, {
    url: "http://api.test/api",
    fetch: (async (input: string | URL | Request, init?: RequestInit) => {
      seen.push({ url: String(input), init: init ?? {} });
      return new Response(JSON.stringify({ id: 1 }), {
        status: 200,
        headers: { "content-type": "application/json" },
      });
    }) as unknown as typeof fetch,
  }) as unknown as RestClient;
}

describe("a generated client calling custom paths", () => {
  it("sends a GET to the resolved path with no input parameter", async () => {
    const seen: { url: string; init: RequestInit }[] = [];
    await restClient(seen).invoices.get({ id: 42 });
    expect(first(seen).url).toBe("http://api.test/api/invoices/42");
    expect(first(seen).init.method).toBe("GET");
  });

  it("keeps unconsumed fields in the query string", async () => {
    const seen: { url: string; init: RequestInit }[] = [];
    await restClient(seen).invoices.list({ limit: 20 });
    expect(first(seen).url).toBe(
      `http://api.test/api/invoices?input=${encodeURIComponent('{"limit":20}')}`,
    );
  });

  it("sends a POST body to the resolved path", async () => {
    const seen: { url: string; init: RequestInit }[] = [];
    await restClient(seen).invoices.create({ total: "USD 3.00" });
    expect(first(seen).url).toBe("http://api.test/api/invoices");
    expect(first(seen).init.body).toBe('{"total":"USD 3.00"}');
  });

  it("resolves several parameters", async () => {
    const seen: { url: string; init: RequestInit }[] = [];
    await restClient(seen).invoices.line({ invoiceId: 3, lineId: "abc" });
    expect(first(seen).url).toBe("http://api.test/api/invoices/3/lines/abc");
  });
});
