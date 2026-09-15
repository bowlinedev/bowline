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
import useSWR, { type SWRConfiguration, type SWRResponse } from "swr";
import useSWRMutation, {
  type SWRMutationConfiguration,
  type SWRMutationResponse,
} from "swr/mutation";

export type MutationKey = readonly [readonly string[]];

export interface QueryHelpers<I, O> {
  key(input?: I): ProcedureKey<I>;
  useQuery: Record<string, never> extends I
    ? (
        input?: I,
        config?: SWRConfiguration<O, BowlineError>,
        call?: CallOptions,
      ) => SWRResponse<O, BowlineError>
    : (
        input: I,
        config?: SWRConfiguration<O, BowlineError>,
        call?: CallOptions,
      ) => SWRResponse<O, BowlineError>;
}

export interface MutationHelpers<I, O> {
  key: MutationKey;
  useMutation(
    config?: SWRMutationConfiguration<O, BowlineError, MutationKey, I>,
    call?: CallOptions,
  ): SWRMutationResponse<O, BowlineError, MutationKey, I>;
}

export type BowlineSWR<C> = {
  [K in keyof C]: C[K] extends { readonly kind: "query" }
    ? QueryHelpers<InputOf<C[K]>, OutputOf<C[K]>>
    : C[K] extends { readonly kind: "mutation" }
      ? MutationHelpers<InputOf<C[K]>, OutputOf<C[K]>>
      : C[K] extends object
        ? BowlineSWR<C[K]>
        : never;
};

export function bowlineSWR<C extends object>(client: C): BowlineSWR<C> {
  return walkProcedures(client as Record<string, unknown>, [], (fn, path) => {
    switch (fn.kind) {
      case "query":
        return queryHelpers(fn, path);
      case "mutation":
        return mutationHelpers(fn, path);
      default:
        return undefined;
    }
  }) as BowlineSWR<C>;
}

function queryHelpers(fn: ProcedureFn, path: readonly string[]): QueryHelpers<unknown, unknown> {
  return {
    key: (input?: unknown) => procedureKey(path, input),
    useQuery: (
      input?: unknown,
      config?: SWRConfiguration<unknown, BowlineError>,
      call?: CallOptions,
    ) => useSWR<unknown, BowlineError>(procedureKey(path, input), () => fn(input, call), config),
  };
}

function mutationHelpers(
  fn: ProcedureFn,
  path: readonly string[],
): MutationHelpers<unknown, unknown> {
  const key: MutationKey = [path];
  return {
    key,
    useMutation: (
      config?: SWRMutationConfiguration<unknown, BowlineError, MutationKey, unknown>,
      call?: CallOptions,
    ) =>
      useSWRMutation<unknown, BowlineError, MutationKey, unknown>(
        key,
        (_key, { arg }) => fn(arg, call),
        config,
      ),
  };
}
