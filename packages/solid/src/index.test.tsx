import { BowlineError, type Mutation, type Query } from "@bowlinedev/client";
import { cleanup, render, screen, waitFor } from "@solidjs/testing-library";
import { createRoot, type ResourceReturn, Show } from "solid-js";
import { afterEach, describe, expect, expectTypeOf, it } from "vitest";
import { type BowlineSolid, bowlineSolid } from "./index.js";

afterEach(cleanup);

interface User {
  id: number;
  name: string;
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
      if (input?.id === 404) {
        throw new BowlineError("NOT_FOUND", "no such user", 404);
      }
      return { id: input?.id ?? 0, name: "Ada" };
    },
    { kind: "query" as const },
  );
  const create = Object.assign(
    async (input: { name: string }) => {
      log.push(`create:${input.name}`);
      if (input.name === "") {
        throw new BowlineError("INVALID_ARGUMENT", "name is required", 400);
      }
      return { id: 9, name: input.name };
    },
    { kind: "mutation" as const },
  );
  const health = Object.assign(async () => ({ ok: true }), { kind: "query" as const });
  return { users: { get, create }, health } as unknown as Client;
}

describe("bowlineSolid", () => {
  it("builds keys shaped like bowlineQuery", () => {
    const bs = bowlineSolid(fakeClient([]));
    expect(bs.users.get.key({ id: 1 })).toEqual([["users", "get"], { id: 1 }]);
    expect(bs.users.get.key()).toEqual([["users", "get"]]);
    expect(bs.users.create.key).toEqual([["users", "create"]]);
    expect(bs.health.key()).toEqual([["health"]]);
  });

  it("renders a resource with data", async () => {
    const log: string[] = [];
    const bs = bowlineSolid(fakeClient(log));
    render(() => {
      const [user] = bs.users.get.resource({ id: 3 });
      return (
        <Show when={user()} fallback={<p data-testid="state">loading</p>}>
          {(u) => <p data-testid="state">{u().name}</p>}
        </Show>
      );
    });
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("Ada"));
    expect(log).toEqual(["get:3"]);
  });

  it("exposes BowlineError on the resource", async () => {
    const bs = bowlineSolid(fakeClient([]));
    render(() => {
      const [user] = bs.users.get.resource(() => ({ id: 404 }));
      return (
        <p data-testid="state">
          {user.error instanceof BowlineError ? `${user.error.code}` : user.state}
        </p>
      );
    });
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("NOT_FOUND"));
  });

  it("tracks action state", async () => {
    const log: string[] = [];
    const bs = bowlineSolid(fakeClient(log));
    await createRoot(async (dispose) => {
      const [mutate, state] = bs.users.create.action();
      expect(state().pending).toBe(false);
      const pending = mutate({ name: "Grace" });
      expect(state().pending).toBe(true);
      const created = await pending;
      expect(created).toEqual({ id: 9, name: "Grace" });
      expect(state()).toEqual({ pending: false });
      await expect(mutate({ name: "" })).rejects.toBeInstanceOf(BowlineError);
      expect(state().error?.code).toBe("INVALID_ARGUMENT");
      expect(log).toEqual(["create:Grace", "create:"]);
      dispose();
    });
  });

  it("infers input and output types", () => {
    type Solid = BowlineSolid<Client>;
    expectTypeOf<Parameters<Solid["users"]["get"]["key"]>[0]>().toEqualTypeOf<
      { id: number } | undefined
    >();
    expectTypeOf<ReturnType<Solid["users"]["get"]["resource"]>>().toEqualTypeOf<
      ResourceReturn<User>
    >();
    expectTypeOf<ReturnType<Solid["users"]["create"]["action"]>[0]>()
      .parameter(0)
      .toEqualTypeOf<{ name: string }>();
    expectTypeOf<
      ReturnType<Solid["users"]["create"]["action"]>[0]
    >().returns.resolves.toEqualTypeOf<User>();
  });
});
