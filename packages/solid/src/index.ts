import {
  type BowlineError,
  type CallOptions,
  type InputOf,
  type OutputOf,
  type ProcedureFn,
  type ProcedureKey,
  procedureKey,
  walkProcedures,
} from "@bowline/client";
import { type Accessor, createResource, createSignal, type ResourceReturn } from "solid-js";

export interface QueryHelpers<I, O> {
  key(input?: I): ProcedureKey<I>;
  resource(input: Accessor<I> | I, options?: { call?: CallOptions }): ResourceReturn<O>;
}

export interface ActionState {
  pending: boolean;
  error?: BowlineError;
}

export interface MutationHelpers<I, O> {
  key: ProcedureKey<never>;
  action(options?: { call?: CallOptions }): [(input: I) => Promise<O>, Accessor<ActionState>];
}

export type BowlineSolid<C> = {
  [K in keyof C]: C[K] extends { readonly kind: "query" }
    ? QueryHelpers<InputOf<C[K]>, OutputOf<C[K]>>
    : C[K] extends { readonly kind: "mutation" }
      ? MutationHelpers<InputOf<C[K]>, OutputOf<C[K]>>
      : C[K] extends object
        ? BowlineSolid<C[K]>
        : never;
};

export function bowlineSolid<C extends object>(client: C): BowlineSolid<C> {
  return walkProcedures(client as Record<string, unknown>, [], (fn, path) => {
    switch (fn.kind) {
      case "query":
        return queryHelpers(fn, path);
      case "mutation":
        return mutationHelpers(fn, path);
      default:
        return undefined;
    }
  }) as BowlineSolid<C>;
}

function isBowlineError(error: unknown): error is BowlineError {
  return error instanceof Error && error.name === "BowlineError";
}

function queryHelpers(fn: ProcedureFn, path: readonly string[]): QueryHelpers<unknown, unknown> {
  return {
    key: (input?: unknown) => procedureKey(path, input),
    resource: (input: Accessor<unknown> | unknown, options?: { call?: CallOptions }) => {
      const source = typeof input === "function" ? (input as Accessor<unknown>) : () => input;
      return createResource(
        () => source() ?? {},
        (value) => fn(value, options?.call),
      );
    },
  };
}

function mutationHelpers(
  fn: ProcedureFn,
  path: readonly string[],
): MutationHelpers<unknown, unknown> {
  return {
    key: [path],
    action: (options?: { call?: CallOptions }) => {
      const [state, setState] = createSignal<ActionState>({ pending: false });
      const mutate = async (input: unknown) => {
        setState({ pending: true });
        try {
          const output = await fn(input, options?.call);
          setState({ pending: false });
          return output;
        } catch (error) {
          if (isBowlineError(error)) {
            setState({ pending: false, error });
          } else {
            setState({ pending: false });
          }
          throw error;
        }
      };
      return [mutate, state];
    },
  };
}
