export type {
  BowlineErrorOptions,
  Code,
  Issue,
  Result,
  TypedError,
  UntypedError,
} from "./error.js";
export { BowlineError } from "./error.js";
export type { ServerEvent } from "./sse.js";
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
  Subscription,
  SubscriptionHandlers,
} from "./types.js";

export const version = "0.1.0";
export { createClient } from "./client.js";
