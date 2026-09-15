import { BowlineError, type Mutation, type Query } from "@bowline/client";
import { get } from "svelte/store";
import { describe, expect, expectTypeOf, it } from "vitest";
import { bowlineStores, type QueryState } from "./stores.js";

type Invoice = { id: number; total: string };
type Client = {
  invoices: {
    get: Query<{ id: number }, Invoice>;
    create: Mutation<{ total: string }, Invoice>;
  };
};

function fakeClient(calls: unknown[]): Client {
  const get = Object.assign(
    async (input?: unknown) => {
      calls.push(input);
      const id = (input as { id: number }).id;
      if (id === 404) {
        throw new BowlineError("NOT_FOUND", "invoice 404 not found", 404);
      }
      if (id === 500) {
        throw new TypeError("network down");
      }
      return { id, total: "USD 1.00" };
    },
    { kind: "query" as const },
  );
  const create = Object.assign(
    async (input?: unknown) => {
      const total = (input as { total: string }).total;
      if (total === "bad") {
        throw new BowlineError("INVALID_ARGUMENT", "invalid input", 400);
      }
      return { id: 9, total };
    },
    { kind: "mutation" as const },
  );
  return { invoices: { get, create } } as unknown as Client;
}

async function settle(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

describe("bowlineStores", () => {
  it("builds keys shaped like bowlineQuery", () => {
    const stores = bowlineStores(fakeClient([]));
    expect(stores.invoices.get.key({ id: 3 })).toEqual([["invoices", "get"], { id: 3 }]);
    expect(stores.invoices.get.key()).toEqual([["invoices", "get"]]);
    expect(stores.invoices.create.key).toEqual([["invoices", "create"]]);
  });

  it("moves from loading to data on first subscribe", async () => {
    const calls: unknown[] = [];
    const store = bowlineStores(fakeClient(calls)).invoices.get.query({ id: 3 });
    const seen: QueryState<Invoice>[] = [];
    const stop = store.subscribe((state) => seen.push(state));
    expect(seen[0]).toEqual({ loading: true });
    await settle();
    expect(seen.at(-1)).toEqual({ data: { id: 3, total: "USD 1.00" }, loading: false });
    expect(calls).toEqual([{ id: 3 }]);
    await store.refresh();
    expect(calls).toHaveLength(2);
    expect(get(store).loading).toBe(false);
    stop();
  });

  it("surfaces BowlineError as state and rethrows other errors", async () => {
    const stores = bowlineStores(fakeClient([]));
    const missing = stores.invoices.get.query({ id: 404 });
    const stop = missing.subscribe(() => {});
    await settle();
    const state = get(missing);
    expect(state.loading).toBe(false);
    expect(state.error).toBeInstanceOf(BowlineError);
    expect(state.error?.code).toBe("NOT_FOUND");
    stop();
    const broken = stores.invoices.get.query({ id: 500 }, { immediate: false });
    expect(get(broken)).toEqual({ loading: false });
    await expect(broken.refresh()).rejects.toThrow("network down");
    expect(get(broken).loading).toBe(false);
  });

  it("tracks pending and errors on mutations", async () => {
    const handle = bowlineStores(fakeClient([])).invoices.create.mutation();
    const pending: boolean[] = [];
    const stop = handle.state.subscribe((state) => pending.push(state.pending));
    const promise = handle.mutate({ total: "USD 2.00" });
    expect(pending).toEqual([false, true]);
    await expect(promise).resolves.toEqual({ id: 9, total: "USD 2.00" });
    expect(get(handle.state)).toEqual({ pending: false });
    await expect(handle.mutate({ total: "bad" })).rejects.toBeInstanceOf(BowlineError);
    expect(get(handle.state).error?.code).toBe("INVALID_ARGUMENT");
    stop();
  });

  it("infers input and output types", () => {
    const stores = bowlineStores(fakeClient([]));
    expectTypeOf(stores.invoices.get.query).parameter(0).toEqualTypeOf<{ id: number }>();
    expectTypeOf(get(stores.invoices.get.query({ id: 1 })).data).toEqualTypeOf<
      Invoice | undefined
    >();
    expectTypeOf(
      stores.invoices.create.mutation().mutate,
    ).returns.resolves.toEqualTypeOf<Invoice>();
  });
});
