import type { CallOptions } from "@bowline/client";

type Brand = {
  readonly kind: "query" | "mutation";
  readonly types?: { readonly input: unknown; readonly output: unknown };
};

type InputOf<T> = T extends { readonly types?: { readonly input: infer I } } ? I : never;
type OutputOf<T> = T extends { readonly types?: { readonly output: infer O } } ? O : never;

export type QueryKeyOf<I> = readonly [readonly string[]] | readonly [readonly string[], I];

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

type Callable = ((input?: unknown, options?: CallOptions) => Promise<unknown>) & Brand;

export function bowlineQuery<C extends object>(client: C): BowlineQuery<C> {
  return build(client as Record<string, unknown>, []) as BowlineQuery<C>;
}

function build(node: Record<string, unknown>, path: string[]): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(node)) {
    const next = [...path, key];
    if (typeof value === "function" && "kind" in value) {
      const fn = value as Callable;
      out[key] = fn.kind === "query" ? queryHelpers(fn, next) : mutationHelpers(fn, next);
    } else if (value && typeof value === "object") {
      out[key] = build(value as Record<string, unknown>, next);
    }
  }
  return out;
}

function queryHelpers(fn: Callable, path: string[]): QueryHelpers<unknown, unknown> {
  const queryKey = (input?: unknown): QueryKeyOf<unknown> =>
    input === undefined ? [path] : [path, input];
  return {
    queryKey,
    queryOptions: (input?: unknown, options?: { call?: CallOptions }) => ({
      queryKey: queryKey(input),
      queryFn: (context: QueryFnContext) =>
        fn(input, { ...options?.call, signal: options?.call?.signal ?? context.signal }),
    }),
  };
}

function mutationHelpers(fn: Callable, path: string[]): MutationHelpers<unknown, unknown> {
  return {
    mutationKey: [path],
    mutationOptions: (options?: { call?: CallOptions }) => ({
      mutationKey: [path],
      mutationFn: (input: unknown) => fn(input, options?.call),
    }),
  };
}
