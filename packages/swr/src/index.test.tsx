import { BowlineError, type Mutation, type Query } from "@bowline/client";
import { render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { useEffect } from "react";
import { type Cache, SWRConfig, type SWRResponse, unstable_serialize } from "swr";
import { describe, expect, expectTypeOf, it } from "vitest";
import { bowlineSWR } from "./index.js";

type User = { id: number; name: string };
type GetInput = { id: number };

const get = Object.assign(
  async (input?: unknown) => ({ id: (input as GetInput).id, name: "Ada" }),
  { kind: "query" as const },
) as unknown as Query<GetInput, User>;

const missing = Object.assign(
  async () => {
    throw new BowlineError("NOT_FOUND", "no such user", 404);
  },
  { kind: "query" as const },
) as unknown as Query<GetInput, User>;

const rename = Object.assign(
  async (input?: unknown) => ({ id: 1, name: (input as { name: string }).name }),
  { kind: "mutation" as const },
) as unknown as Mutation<{ name: string }, User>;

const watch = Object.assign(async () => undefined, { kind: "subscription" as const });

const client = { users: { get, missing, rename, watch } };
const swr = bowlineSWR(client);

function fresh(children: ReactNode, cache: Cache) {
  return <SWRConfig value={{ provider: () => cache, dedupingInterval: 0 }}>{children}</SWRConfig>;
}

describe("bowlineSWR", () => {
  it("renders query data under the procedure key", async () => {
    const cache: Cache = new Map();
    function Profile() {
      const { data } = swr.users.get.useQuery({ id: 1 });
      return <p>{data ? data.name : "loading"}</p>;
    }
    render(fresh(<Profile />, cache));
    await waitFor(() => expect(screen.getByText("Ada")).toBeDefined());
    expect(swr.users.get.key({ id: 1 })).toEqual([["users", "get"], { id: 1 }]);
    expect(swr.users.get.key()).toEqual([["users", "get"]]);
    expect([...cache.keys()]).toContain(unstable_serialize([["users", "get"], { id: 1 }]));
  });

  it("surfaces a BowlineError with its code", async () => {
    function Broken() {
      const { error } = swr.users.missing.useQuery({ id: 9 });
      return <p>{error instanceof BowlineError ? error.code : "pending"}</p>;
    }
    render(fresh(<Broken />, new Map()));
    await waitFor(() => expect(screen.getByText("NOT_FOUND")).toBeDefined());
  });

  it("triggers mutations and exposes the result", async () => {
    function Rename() {
      const { trigger, data } = swr.users.rename.useMutation();
      useEffect(() => {
        void trigger({ name: "Grace" });
      }, [trigger]);
      return <p>{data ? data.name : "idle"}</p>;
    }
    render(fresh(<Rename />, new Map()));
    await waitFor(() => expect(screen.getByText("Grace")).toBeDefined());
    expect(swr.users.rename.key).toEqual([["users", "rename"]]);
  });

  it("skips subscriptions and types the helpers", () => {
    expect("watch" in swr.users).toBe(false);
    expectTypeOf(swr.users.get.useQuery).returns.toEqualTypeOf<SWRResponse<User, BowlineError>>();
    expectTypeOf(swr.users.get.key).parameter(0).toEqualTypeOf<GetInput | undefined>();
    type Trigger = ReturnType<typeof swr.users.rename.useMutation>["trigger"];
    expectTypeOf<Parameters<Trigger>[0]>().toEqualTypeOf<{ name: string }>();
    expectTypeOf<Awaited<ReturnType<Trigger>>>().toEqualTypeOf<User | undefined>();
  });
});
