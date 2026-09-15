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
  SubscriptionTransport,
  TransportEvent,
  Upload,
} from "./types.js";
export type { WebSocketTransportOptions } from "./ws.js";
export { websocketTransport } from "./ws.js";

export const version = "0.2.0";
export { createClient } from "./client.js";
