import {
  BowlineError,
  type CallOptions,
  type InputOf,
  type OutputOf,
  type ProcedureFn,
  type ProcedureKey,
  procedureKey,
  walkProcedures,
} from "@bowline/client";
import { type Readable, readable, writable } from "svelte/store";

export interface QueryState<O> {
  data?: O;
  error?: BowlineError;
  loading: boolean;
}

export interface QueryOptions {
  call?: CallOptions;
  immediate?: boolean;
}

export type QueryStore<O> = Readable<QueryState<O>> & { refresh(): Promise<void> };

export interface MutationState {
  pending: boolean;
  error?: BowlineError;
}

export interface MutationHandle<I, O> {
  mutate(input: I): Promise<O>;
  state: Readable<MutationState>;
}

export interface QueryStores<I, O> {
  key(input?: I): ProcedureKey<I>;
  query: Record<string, never> extends I
    ? (input?: I, options?: QueryOptions) => QueryStore<O>
    : (input: I, options?: QueryOptions) => QueryStore<O>;
}

export interface MutationStores<I, O> {
  key: ProcedureKey<never>;
  mutation(options?: { call?: CallOptions }): MutationHandle<I, O>;
}

export type BowlineStores<C> = {
  [K in keyof C]: C[K] extends { readonly kind: "query" }
    ? QueryStores<InputOf<C[K]>, OutputOf<C[K]>>
    : C[K] extends { readonly kind: "mutation" }
      ? MutationStores<InputOf<C[K]>, OutputOf<C[K]>>
      : C[K] extends object
        ? BowlineStores<C[K]>
        : never;
};

export function bowlineStores<C extends object>(client: C): BowlineStores<C> {
  return walkProcedures(client as Record<string, unknown>, [], (fn, path) => {
    switch (fn.kind) {
      case "query":
        return queryStores(fn, path);
      case "mutation":
        return mutationStores(fn, path);
      default:
        return undefined;
    }
  }) as BowlineStores<C>;
}

function queryStores(fn: ProcedureFn, path: readonly string[]): QueryStores<unknown, unknown> {
  return {
    key: (input?: unknown) => procedureKey(path, input),
    query: (input?: unknown, options?: QueryOptions) => queryStore(fn, input, options),
  };
}

function queryStore(fn: ProcedureFn, input: unknown, options?: QueryOptions): QueryStore<unknown> {
  const immediate = options?.immediate ?? true;
  let current: QueryState<unknown> = { loading: immediate };
  let set: ((state: QueryState<unknown>) => void) | undefined;
  const update = (state: QueryState<unknown>) => {
    current = state;
    set?.(state);
  };
  const refresh = async () => {
    update({ ...current, loading: true });
    try {
      const data = await fn(input, options?.call);
      update({ data, loading: false });
    } catch (error) {
      if (error instanceof BowlineError) {
        update({ error, loading: false });
        return;
      }
      update({ ...current, loading: false });
      throw error;
    }
  };
  let started = false;
  const store = readable(current, (setter) => {
    set = setter;
    setter(current);
    if (immediate && !started) {
      started = true;
      void refresh().catch(() => {});
    }
    return () => {
      set = undefined;
    };
  });
  return { subscribe: store.subscribe, refresh };
}

function mutationStores(
  fn: ProcedureFn,
  path: readonly string[],
): MutationStores<unknown, unknown> {
  return {
    key: [path],
    mutation: (options?: { call?: CallOptions }) => {
      const state = writable<MutationState>({ pending: false });
      return {
        state: { subscribe: state.subscribe },
        mutate: async (input: unknown) => {
          state.set({ pending: true });
          try {
            const result = await fn(input, options?.call);
            state.set({ pending: false });
            return result;
          } catch (error) {
            if (error instanceof BowlineError) {
              state.set({ pending: false, error });
            } else {
              state.set({ pending: false });
            }
            throw error;
          }
        },
      };
    },
  };
}
