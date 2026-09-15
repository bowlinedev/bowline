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

export type Query<I, O> = Callable<I, O> & {
  readonly kind: "query";
  readonly types?: ProcedureTypes<I, O>;
};

export type Mutation<I, O> = Callable<I, O> & {
  readonly kind: "mutation";
  readonly types?: ProcedureTypes<I, O>;
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
}

export interface ContractRuntime {
  version: string;
  hydrators: Record<string, HydrateEntry[]>;
  procedures: Record<string, ProcedureRuntime>;
}
