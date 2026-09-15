import type { BowlineError, Result } from "./error.js";

export type Base64 = string & { readonly __bowline: "base64" };

export type DurationNs = number & { readonly __bowline: "duration-ns" };

export interface CallOptions {
  signal?: AbortSignal;
  headers?: HeadersInit;
}

export type HeadersSource = HeadersInit | (() => HeadersInit | Promise<HeadersInit>);

export interface ClientOptions {
  url: string;
  fetch?: typeof fetch;
  headers?: HeadersSource;
}

type Callable<I, O> = Record<string, never> extends I
  ? (input?: I, options?: CallOptions) => Promise<O>
  : (input: I, options?: CallOptions) => Promise<O>;

export interface ProcedureTypes<I, O> {
  readonly input: I;
  readonly output: O;
}

interface Procedure<I, O, E> {
  readonly types?: ProcedureTypes<I, O>;
  readonly safe: Callable<I, Result<O, E>>;
}

export type Query<I, O, E = BowlineError> = Callable<I, O> &
  Procedure<I, O, E> & {
    readonly kind: "query";
  };

export type Mutation<I, O, E = BowlineError> = Callable<I, O> &
  Procedure<I, O, E> & {
    readonly kind: "mutation";
  };

export type HydrateKind = "timestamp" | "bigint" | { ref: string };

export interface HydrateEntry {
  path: string[];
  kind: HydrateKind;
}

export interface ProcedureRuntime {
  kind: "query" | "mutation";
  method: "GET" | "POST";
  output?: string;
  errors?: string[];
}

export interface ContractRuntime {
  version: string;
  hydrators: Record<string, HydrateEntry[]>;
  procedures: Record<string, ProcedureRuntime>;
}
