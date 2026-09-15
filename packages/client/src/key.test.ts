import { describe, expect, expectTypeOf, it } from "vitest";
import { type InputOf, type OutputOf, procedureKey, walkProcedures } from "./key.js";
import type { Query } from "./types.js";

describe("procedureKey", () => {
  it("omits the input slot when the input is undefined", () => {
    const path = ["users", "get"];
    expect(procedureKey(path)).toEqual([["users", "get"]]);
    expect(procedureKey(path, { id: 1 })).toEqual([["users", "get"], { id: 1 }]);
  });

  it("keeps the same path reference so keys hash stably", () => {
    const path = ["users", "list"];
    const key = procedureKey(path, { limit: 2 });
    expect(key[0]).toBe(path);
  });

  it("infers input and output types from branded procedures", () => {
    type Get = Query<{ id: number }, { name: string }>;
    expectTypeOf<InputOf<Get>>().toEqualTypeOf<{ id: number }>();
    expectTypeOf<OutputOf<Get>>().toEqualTypeOf<{ name: string }>();
  });

  it("walks a client tree and skips nodes the visitor rejects", () => {
    const get = Object.assign(async () => ({}), { kind: "query" as const });
    const create = Object.assign(async () => ({}), { kind: "mutation" as const });
    const tree = { users: { get, create }, health: get };
    const seen: string[] = [];
    const built = walkProcedures(tree, [], (fn, path) => {
      seen.push(`${path.join(".")}:${fn.kind}`);
      return fn.kind === "query" ? path.join("/") : undefined;
    });
    expect(seen).toEqual(["users.get:query", "users.create:mutation", "health:query"]);
    expect(built).toEqual({ users: { get: "users/get" }, health: "health" });
  });
});
