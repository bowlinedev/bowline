import {
  type BowlineError,
  type CallOptions,
  type InputOf,
  type OutputOf,
  type ProcedureFn,
  type ProcedureKey,
  procedureKey,
  walkProcedures,
} from "@bowlinedev/client";
import {
  type ComputedRef,
  computed,
  type MaybeRefOrGetter,
  type Ref,
  ref,
  shallowRef,
  toValue,
  watch,
} from "vue";

export interface QueryFnContext {
  signal?: AbortSignal;
}

export interface QueryResult<O> {
  data: Ref<O | undefined>;
  error: Ref<BowlineError | undefined>;
  loading: Ref<boolean>;
  refresh(): Promise<void>;
}

export interface QueryHelpers<I, O> {
  queryKey(input?: I): ProcedureKey<I>;
  queryOptions(
    input: MaybeRefOrGetter<I>,
    options?: { call?: CallOptions },
  ): { queryKey: ComputedRef<ProcedureKey<I>>; queryFn: (context: QueryFnContext) => Promise<O> };
  useQuery(
    input: MaybeRefOrGetter<I>,
    options?: { call?: CallOptions; immediate?: boolean },
  ): QueryResult<O>;
}

export interface MutationResult<I, O> {
  mutate(input: I): Promise<O>;
  pending: Ref<boolean>;
  error: Ref<BowlineError | undefined>;
}

export interface MutationHelpers<I, O> {
  mutationKey: ProcedureKey<never>;
  mutationOptions(options?: { call?: CallOptions }): {
    mutationKey: ProcedureKey<never>;
    mutationFn: (input: I) => Promise<O>;
  };
  useMutation(options?: { call?: CallOptions }): MutationResult<I, O>;
}

export type BowlineVue<C> = {
  [K in keyof C]: C[K] extends { readonly kind: "query" }
    ? QueryHelpers<InputOf<C[K]>, OutputOf<C[K]>>
    : C[K] extends { readonly kind: "mutation" }
      ? MutationHelpers<InputOf<C[K]>, OutputOf<C[K]>>
      : C[K] extends object
        ? BowlineVue<C[K]>
        : never;
};

export function bowlineVue<C extends object>(client: C): BowlineVue<C> {
  return walkProcedures(client as Record<string, unknown>, [], (fn, path) => {
    switch (fn.kind) {
      case "query":
        return queryHelpers(fn, path);
      case "mutation":
        return mutationHelpers(fn, path);
      default:
        return undefined;
    }
  }) as BowlineVue<C>;
}

function isBowlineError(error: unknown): error is BowlineError {
  return error instanceof Error && error.name === "BowlineError";
}

function queryHelpers(fn: ProcedureFn, path: readonly string[]): QueryHelpers<unknown, unknown> {
  return {
    queryKey: (input?: unknown) => procedureKey(path, input),
    queryOptions: (input: MaybeRefOrGetter<unknown>, options?: { call?: CallOptions }) => ({
      queryKey: computed(() => procedureKey(path, toValue(input))),
      queryFn: (context: QueryFnContext) =>
        fn(toValue(input), { ...options?.call, signal: options?.call?.signal ?? context.signal }),
    }),
    useQuery: (
      input: MaybeRefOrGetter<unknown>,
      options?: { call?: CallOptions; immediate?: boolean },
    ) => {
      const data = shallowRef<unknown>();
      const error = shallowRef<BowlineError | undefined>();
      const loading = ref(false);
      let generation = 0;
      const refresh = async () => {
        const current = ++generation;
        loading.value = true;
        try {
          const output = await fn(toValue(input), options?.call);
          if (current === generation) {
            data.value = output;
            error.value = undefined;
          }
        } catch (caught) {
          if (current === generation) {
            if (!isBowlineError(caught)) {
              throw caught;
            }
            error.value = caught;
          }
        } finally {
          if (current === generation) {
            loading.value = false;
          }
        }
      };
      watch(
        () => toValue(input),
        () => {
          void refresh();
        },
        { immediate: options?.immediate ?? true, deep: true },
      );
      return { data, error, loading, refresh };
    },
  };
}

function mutationHelpers(
  fn: ProcedureFn,
  path: readonly string[],
): MutationHelpers<unknown, unknown> {
  const mutationKey: ProcedureKey<never> = [path];
  return {
    mutationKey,
    mutationOptions: (options?: { call?: CallOptions }) => ({
      mutationKey,
      mutationFn: (input: unknown) => fn(input, options?.call),
    }),
    useMutation: (options?: { call?: CallOptions }) => {
      const pending = ref(false);
      const error = shallowRef<BowlineError | undefined>();
      const mutate = async (input: unknown) => {
        pending.value = true;
        error.value = undefined;
        try {
          return await fn(input, options?.call);
        } catch (caught) {
          if (isBowlineError(caught)) {
            error.value = caught;
          }
          throw caught;
        } finally {
          pending.value = false;
        }
      };
      return { mutate, pending, error };
    },
  };
}
