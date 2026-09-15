import { BowlineError, type Mutation, type Query } from "@bowline/client";
import { QueryClient, useQuery as useTanstackQuery, VueQueryPlugin } from "@tanstack/vue-query";
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, expectTypeOf, it } from "vitest";
import { defineComponent, h, nextTick, ref } from "vue";
import { type BowlineVue, bowlineVue } from "./index.js";

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

describe("bowlineVue", () => {
  it("builds keys shaped like bowlineQuery", () => {
    const bv = bowlineVue(fakeClient([]));
    expect(bv.users.get.queryKey({ id: 1 })).toEqual([["users", "get"], { id: 1 }]);
    expect(bv.users.get.queryKey()).toEqual([["users", "get"]]);
    expect(bv.users.create.mutationKey).toEqual([["users", "create"]]);
    expect(bv.health.queryKey()).toEqual([["health"]]);
  });

  it("useQuery loads data, reacts to input changes, and exposes errors", async () => {
    const log: string[] = [];
    const bv = bowlineVue(fakeClient(log));
    const id = ref(3);
    const wrapper = mount(
      defineComponent({
        setup() {
          const { data, error, loading } = bv.users.get.useQuery(() => ({ id: id.value }));
          return () =>
            h(
              "p",
              { "data-testid": "state" },
              error.value ? error.value.code : loading.value ? "loading" : (data.value?.name ?? ""),
            );
        },
      }),
    );
    expect(wrapper.text()).toBe("loading");
    await flushPromises();
    expect(wrapper.text()).toBe("Ada");
    id.value = 404;
    await nextTick();
    await flushPromises();
    expect(wrapper.text()).toBe("NOT_FOUND");
    expect(log).toEqual(["get:3", "get:404"]);
  });

  it("useMutation tracks pending and error state", async () => {
    const log: string[] = [];
    const bv = bowlineVue(fakeClient(log));
    const { mutate, pending, error } = bv.users.create.useMutation();
    const promise = mutate({ name: "Grace" });
    expect(pending.value).toBe(true);
    expect(await promise).toEqual({ id: 9, name: "Grace" });
    expect(pending.value).toBe(false);
    await expect(mutate({ name: "" })).rejects.toBeInstanceOf(BowlineError);
    expect(error.value?.code).toBe("INVALID_ARGUMENT");
    expect(log).toEqual(["create:Grace", "create:"]);
  });

  it("feeds queryOptions into TanStack Vue Query", async () => {
    const log: string[] = [];
    const bv = bowlineVue(fakeClient(log));
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = mount(
      defineComponent({
        setup() {
          const options = bv.users.get.queryOptions({ id: 1 });
          const query = useTanstackQuery(options);
          return () =>
            h("p", { "data-testid": "state" }, [
              JSON.stringify(options.queryKey.value),
              "|",
              query.data.value?.name ?? "",
            ]);
        },
      }),
      { global: { plugins: [[VueQueryPlugin, { queryClient }]] } },
    );
    await flushPromises();
    expect(wrapper.text()).toBe('[["users","get"],{"id":1}]|Ada');
    expect(queryClient.getQueryData([["users", "get"], { id: 1 }])).toEqual({ id: 1, name: "Ada" });
  });

  it("infers input and output types", () => {
    type Vue = BowlineVue<Client>;
    expectTypeOf<Parameters<Vue["users"]["get"]["queryKey"]>[0]>().toEqualTypeOf<
      { id: number } | undefined
    >();
    expectTypeOf<ReturnType<Vue["users"]["get"]["useQuery"]>["data"]["value"]>().toEqualTypeOf<
      User | undefined
    >();
    expectTypeOf<ReturnType<Vue["users"]["create"]["useMutation"]>["mutate"]>()
      .parameter(0)
      .toEqualTypeOf<{ name: string }>();
    expectTypeOf<
      ReturnType<Vue["users"]["create"]["mutationOptions"]>["mutationFn"]
    >().returns.resolves.toEqualTypeOf<User>();
  });
});
