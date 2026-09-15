import type { Mutation, Query } from "@bowline/client";
import { expect, expectTypeOf, test } from "vitest";
import { bowlineQuery } from "./index.js";

interface User {
  id: number;
}

interface Client {
  users: {
    get: Query<{ id: number }, User>;
    create: Mutation<{ name: string }, User>;
  };
  health: Query<Record<string, never>, { ok: boolean }>;
}

function fakeClient(log: string[]): Client {
  const get = Object.assign(
    async (input?: { id: number }) => {
      log.push(`get:${input?.id}`);
      return { id: input?.id ?? 0 };
    },
    { kind: "query" as const },
  );
  const create = Object.assign(
    async (input: { name: string }) => {
      log.push(`create:${input.name}`);
      return { id: 1 };
    },
    { kind: "mutation" as const },
  );
  const health = Object.assign(async () => ({ ok: true }), { kind: "query" as const });
  return { users: { get, create }, health } as unknown as Client;
}

test("builds query options with stable keys", async () => {
  const log: string[] = [];
  const bq = bowlineQuery(fakeClient(log));
  const options = bq.users.get.queryOptions({ id: 7 });
  expect(options.queryKey).toEqual([["users", "get"], { id: 7 }]);
  expect(bq.users.get.queryKey()).toEqual([["users", "get"]]);
  expect(bq.users.get.queryKey({ id: 7 })).toEqual([["users", "get"], { id: 7 }]);
  const result = await options.queryFn({ signal: new AbortController().signal });
  expect(result).toEqual({ id: 7 });
  expect(log).toEqual(["get:7"]);
  expectTypeOf(result).toEqualTypeOf<User>();
});

test("builds mutation options", async () => {
  const log: string[] = [];
  const bq = bowlineQuery(fakeClient(log));
  const options = bq.users.create.mutationOptions();
  expect(options.mutationKey).toEqual([["users", "create"]]);
  expect(await options.mutationFn({ name: "ada" })).toEqual({ id: 1 });
  expectTypeOf(options.mutationFn).parameter(0).toEqualTypeOf<{ name: string }>();
});

test("optional inputs stay optional", async () => {
  const bq = bowlineQuery(fakeClient([]));
  expect(bq.health.queryOptions().queryKey).toEqual([["health"]]);
  expect(await bq.health.queryOptions().queryFn({ signal: new AbortController().signal })).toEqual({
    ok: true,
  });
});
