export type { BowlineErrorOptions, Code, Issue } from "./error.js";
export { BowlineError } from "./error.js";
export type {
  Base64,
  CallOptions,
  ClientOptions,
  ContractRuntime,
  DurationNs,
  HeadersSource,
  HydrateEntry,
  HydrateKind,
  Mutation,
  ProcedureRuntime,
  ProcedureTypes,
  Query,
} from "./types.js";

export const version = "0.0.0";
export { createClient } from "./client.js";
