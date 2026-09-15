import {
  type CallOptions,
  type InputOf,
  type OutputOf,
  type ProcedureFn,
  type ProcedureKey,
  procedureKey,
  walkProcedures,
} from "@bowline/client";

export type QueryKeyOf<I> = ProcedureKey<I>;

export interface QueryFnContext {
  signal: AbortSignal;
}

export interface QueryHelpers<I, O> {
  queryKey(input?: I): QueryKeyOf<I>;
  queryOptions: Record<string, never> extends I
    ? (
        input?: I,
        options?: { call?: CallOptions },
      ) => { queryKey: QueryKeyOf<I>; queryFn: (context: QueryFnContext) => Promise<O> }
    : (
        input: I,
        options?: { call?: CallOptions },
      ) => { queryKey: QueryKeyOf<I>; queryFn: (context: QueryFnContext) => Promise<O> };
}

export interface MutationHelpers<I, O> {
  mutationKey: readonly [readonly string[]];
  mutationOptions(options?: { call?: CallOptions }): {
    mutationKey: readonly [readonly string[]];
    mutationFn: (input: I) => Promise<O>;
  };
}

export type BowlineQuery<C> = {
  [K in keyof C]: C[K] extends { readonly kind: "query" }
    ? QueryHelpers<InputOf<C[K]>, OutputOf<C[K]>>
    : C[K] extends { readonly kind: "mutation" }
      ? MutationHelpers<InputOf<C[K]>, OutputOf<C[K]>>
      : C[K] extends object
        ? BowlineQuery<C[K]>
        : never;
};

export function bowlineQuery<C extends object>(client: C): BowlineQuery<C> {
  return walkProcedures(client as Record<string, unknown>, [], (fn, path) => {
    switch (fn.kind) {
      case "query":
        return queryHelpers(fn, path);
      case "mutation":
        return mutationHelpers(fn, path);
      default:
        return undefined;
    }
  }) as BowlineQuery<C>;
}

function queryHelpers(fn: ProcedureFn, path: readonly string[]): QueryHelpers<unknown, unknown> {
  return {
    queryKey: (input?: unknown) => procedureKey(path, input),
    queryOptions: (input?: unknown, options?: { call?: CallOptions }) => ({
      queryKey: procedureKey(path, input),
      queryFn: (context: QueryFnContext) =>
        fn(input, { ...options?.call, signal: options?.call?.signal ?? context.signal }),
    }),
  };
}

function mutationHelpers(
  fn: ProcedureFn,
  path: readonly string[],
): MutationHelpers<unknown, unknown> {
  return {
    mutationKey: [path],
    mutationOptions: (options?: { call?: CallOptions }) => ({
      mutationKey: [path],
      mutationFn: (input: unknown) => fn(input, options?.call),
    }),
  };
}
