import { expect, test } from "vitest";
import { hydrate, serialize } from "./hydrate.js";
import type { HydrateEntry } from "./types.js";

const hydrators: Record<string, HydrateEntry[]> = {
  User: [
    { path: ["createdAt"], kind: "timestamp" },
    { path: ["id"], kind: "bigint" },
    { path: ["tags", "*", "at"], kind: "timestamp" },
    { path: ["byKey", "*"], kind: "timestamp" },
  ],
  "Page<User>": [{ path: ["items", "*"], kind: { ref: "User" } }],
  Tree: [
    { path: ["children", "*"], kind: { ref: "Tree" } },
    { path: ["at"], kind: "timestamp" },
  ],
};

test("hydrates timestamps and bigints along paths", () => {
  const value = hydrate(
    {
      createdAt: "2026-09-15T12:00:00Z",
      id: "9007199254740993",
      tags: [{ at: "2026-01-01T00:00:00Z" }, { at: null }],
      byKey: { a: "2026-02-02T00:00:00Z" },
      nickname: null,
    },
    "User",
    hydrators,
  ) as { createdAt: Date; id: bigint; tags: { at: Date | null }[]; byKey: Record<string, Date> };
  expect(value.createdAt).toBeInstanceOf(Date);
  expect(value.createdAt.toISOString()).toBe("2026-09-15T12:00:00.000Z");
  expect(value.id).toBe(9007199254740993n);
  expect(value.tags[0]?.at).toBeInstanceOf(Date);
  expect(value.tags[1]?.at).toBeNull();
  expect(value.byKey.a).toBeInstanceOf(Date);
});

test("follows refs through generics and recursion", () => {
  const page = hydrate(
    { items: [{ createdAt: "2026-09-15T12:00:00Z", id: "1" }] },
    "Page<User>",
    hydrators,
  ) as {
    items: { createdAt: Date }[];
  };
  expect(page.items[0]?.createdAt).toBeInstanceOf(Date);
  const tree = hydrate(
    { at: "2026-01-01T00:00:00Z", children: [{ at: "2026-01-02T00:00:00Z", children: [] }] },
    "Tree",
    hydrators,
  ) as {
    children: { at: Date }[];
  };
  expect(tree.children[0]?.at).toBeInstanceOf(Date);
});

test("ignores unknown keys and missing paths", () => {
  expect(hydrate({ id: "1" }, "Nope", hydrators)).toEqual({ id: "1" });
  expect(hydrate(undefined, "User", hydrators)).toBeUndefined();
  expect(hydrate({ tags: undefined }, "User", hydrators)).toEqual({ tags: undefined });
});

test("serialize handles bigint and Date", () => {
  expect(serialize({ id: 5n, at: new Date("2026-09-15T12:00:00Z"), n: 1 })).toBe(
    '{"id":"5","at":"2026-09-15T12:00:00.000Z","n":1}',
  );
});
