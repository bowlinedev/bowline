import {
  BowlineError,
  type BowlineErrorOptions,
  type Code,
  type Issue,
  type Result,
} from "./error.js";
import { hydrate, serialize } from "./hydrate.js";
import type { RecordSink } from "./record.js";
import { record } from "./record.js";
import { parseEventStream } from "./sse.js";
import type {
  CallOptions,
  ClientOptions,
  ContractRuntime,
  HeadersSource,
  ProcedureRuntime,
  SubscriptionHandlers,
  SubscriptionTransport,
} from "./types.js";

type Leaf = ((input?: unknown, options?: CallOptions) => Promise<unknown>) & {
  kind: "query" | "mutation";
  safe: (input?: unknown, options?: CallOptions) => Promise<Result<unknown, BowlineError>>;
};

type StreamLeaf = ((
  input?: unknown,
  options?: CallOptions,
  onOpen?: () => void,
) => AsyncIterable<unknown>) & {
  kind: "subscription";
  subscribe: (
    input: unknown,
    handlers: SubscriptionHandlers<unknown>,
    options?: CallOptions,
  ) => () => void;
};

export function createClient(contract: ContractRuntime, options: ClientOptions): unknown {
  const base = options.url.replace(/\/+$/, "");
  const fetchFn = options.fetch ?? globalThis.fetch;
  const root: Record<string, unknown> = {};
  for (const [path, proc] of Object.entries(contract.procedures)) {
    const segments = path.split(".");
    let node = root;
    for (const segment of segments.slice(0, -1)) {
      const next = node[segment];
      if (next === undefined) {
        const created: Record<string, unknown> = {};
        node[segment] = created;
        node = created;
      } else {
        node = next as Record<string, unknown>;
      }
    }
    const key = segments[segments.length - 1] as string;
    if (proc.kind === "upload") {
      node[key] = uploadLeaf(fetchFn, base, path, proc, contract, options.headers, options.record);
      continue;
    }
    if (proc.kind === "subscription") {
      node[key] = streamLeaf(
        fetchFn,
        base,
        path,
        proc,
        contract,
        options.headers,
        options.transport,
      );
      continue;
    }
    const leaf = ((input?: unknown, callOptions?: CallOptions) =>
      call(
        fetchFn,
        base,
        path,
        proc,
        contract,
        options.headers,
        input,
        callOptions,
        options.record,
      )) as Leaf;
    leaf.kind = proc.kind;
    leaf.safe = async (input?: unknown, callOptions?: CallOptions) => {
      try {
        return { ok: true, value: await leaf(input, callOptions) };
      } catch (error) {
        if (error instanceof BowlineError) {
          return { ok: false, error };
        }
        throw error;
      }
    };
    node[key] = leaf;
  }
  return root;
}

export function sendsBody(method: string): boolean {
  return method !== "GET" && method !== "DELETE" && method !== "HEAD";
}

export function queryString(payload: unknown): string {
  if (payload === null || typeof payload !== "object") {
    return "";
  }
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(payload as Record<string, unknown>)) {
    if (value === undefined || value === null) {
      continue;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        params.append(key, String(item));
      }
      continue;
    }
    params.append(key, String(value));
  }
  const encoded = params.toString();
  return encoded === "" ? "" : `?${encoded}`;
}

export function resolvePath(
  procedurePath: string,
  proc: ProcedureRuntime,
  input: unknown,
): { path: string; payload: unknown } {
  if (proc.path === undefined) {
    return { path: procedurePath, payload: input };
  }
  const consumed: string[] = [];
  const segments = proc.path.split("/").map((segment) => {
    if (!segment.startsWith("{") || !segment.endsWith("}")) {
      return segment;
    }
    const name = segment.slice(1, -1);
    const value = (input as Record<string, unknown> | undefined)?.[name];
    if (value === undefined || value === null) {
      throw new TypeError(`bowline: path parameter "${name}" is missing from the input`);
    }
    consumed.push(name);
    return encodeURIComponent(String(value));
  });
  if (consumed.length === 0 || input === undefined || input === null) {
    return { path: segments.join("/"), payload: input };
  }
  const rest: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(input as Record<string, unknown>)) {
    if (!consumed.includes(key)) {
      rest[key] = value;
    }
  }
  return {
    path: segments.join("/"),
    payload: Object.keys(rest).length === 0 ? undefined : rest,
  };
}

async function prepare(
  base: string,
  path: string,
  proc: ProcedureRuntime,
  headersSource: HeadersSource | undefined,
  input: unknown,
  callOptions: CallOptions | undefined,
  accept: string,
): Promise<{ url: string; init: RequestInit }> {
  const headers = new Headers(
    typeof headersSource === "function" ? await headersSource() : headersSource,
  );
  for (const [k, v] of new Headers(callOptions?.headers)) {
    headers.set(k, v);
  }
  headers.set("accept", accept);
  if (callOptions?.idempotencyKey !== undefined) {
    headers.set("idempotency-key", callOptions.idempotencyKey);
  }
  const { path: resolved, payload } = resolvePath(path, proc, input);
  const body = serialize(payload ?? {});
  let url = `${base}/${resolved}`;
  const init: RequestInit = { method: proc.method, headers, signal: callOptions?.signal ?? null };
  if (!sendsBody(proc.method)) {
    if (payload !== undefined) {
      url += proc.path === undefined ? `?input=${encodeURIComponent(body)}` : queryString(payload);
    }
  } else {
    headers.set("content-type", "application/json");
    init.body = body;
  }
  return { url, init };
}

async function send(
  fetchFn: typeof fetch,
  url: string,
  init: RequestInit,
  path: string,
  signal: AbortSignal | undefined,
): Promise<Response> {
  try {
    return await fetchFn(url, init);
  } catch (cause) {
    if (signal?.aborted) {
      throw new BowlineError("CANCELED", "request aborted", 0, { cause });
    }
    throw new BowlineError("UNAVAILABLE", `network error calling ${path}`, 0, { cause });
  }
}

async function call(
  fetchFn: typeof fetch,
  base: string,
  path: string,
  proc: ProcedureRuntime,
  contract: ContractRuntime,
  headersSource: HeadersSource | undefined,
  input: unknown,
  callOptions: CallOptions | undefined,
  sink: RecordSink | undefined,
): Promise<unknown> {
  const { url, init } = await prepare(
    base,
    path,
    proc,
    headersSource,
    input,
    callOptions,
    "application/json",
  );
  const response = await send(fetchFn, url, init, path, callOptions?.signal);
  const text = await response.text();
  record(sink, path, proc.method, input, response.status, text);
  if (!response.ok) {
    throw toError(response.status, text, path, contract);
  }
  const data = parseBody(text, response.status, path);
  return proc.output === undefined ? data : hydrate(data, proc.output, contract.hydrators);
}

type UploadLeaf = ((input: unknown, file: Blob, options?: CallOptions) => Promise<unknown>) & {
  kind: "upload";
  safe: (
    input: unknown,
    file: Blob,
    options?: CallOptions,
  ) => Promise<Result<unknown, BowlineError>>;
};

function uploadLeaf(
  fetchFn: typeof fetch,
  base: string,
  path: string,
  proc: ProcedureRuntime,
  contract: ContractRuntime,
  headersSource: HeadersSource | undefined,
  sink: RecordSink | undefined,
): UploadLeaf {
  const leaf = (async (input: unknown, file: Blob, callOptions?: CallOptions) => {
    const headers = new Headers(
      typeof headersSource === "function" ? await headersSource() : headersSource,
    );
    for (const [k, v] of new Headers(callOptions?.headers)) {
      headers.set(k, v);
    }
    headers.set("accept", "application/json");
    headers.delete("content-type");
    const form = new FormData();
    form.append("input", new Blob([serialize(input ?? {})], { type: "application/json" }));
    form.append("file", file, file instanceof File ? file.name : "upload");
    const response = await send(
      fetchFn,
      `${base}/${path}`,
      { method: "POST", headers, body: form, signal: callOptions?.signal ?? null },
      path,
      callOptions?.signal,
    );
    const text = await response.text();
    record(sink, path, "POST", input, response.status, text);
    if (!response.ok) {
      throw toError(response.status, text, path, contract);
    }
    const data = parseBody(text, response.status, path);
    return proc.output === undefined ? data : hydrate(data, proc.output, contract.hydrators);
  }) as UploadLeaf;
  leaf.kind = "upload";
  leaf.safe = async (input, file, callOptions) => {
    try {
      return { ok: true, value: await leaf(input, file, callOptions) };
    } catch (error) {
      if (error instanceof BowlineError) {
        return { ok: false, error };
      }
      throw error;
    }
  };
  return leaf;
}

function streamLeaf(
  fetchFn: typeof fetch,
  base: string,
  path: string,
  proc: ProcedureRuntime,
  contract: ContractRuntime,
  headersSource: HeadersSource | undefined,
  transport: SubscriptionTransport | undefined,
): StreamLeaf {
  const iterate = ((input?: unknown, callOptions?: CallOptions, onOpen?: () => void) =>
    transport
      ? transportEvents(transport, path, proc, contract, input, callOptions, onOpen)
      : streamEvents(
          fetchFn,
          base,
          path,
          proc,
          contract,
          headersSource,
          input,
          callOptions,
          onOpen,
        )) as StreamLeaf;
  iterate.kind = "subscription";
  iterate.subscribe = (input, handlers, callOptions) => {
    const controller = new AbortController();
    callOptions?.signal?.addEventListener("abort", () => controller.abort(), { once: true });
    void (async () => {
      try {
        for await (const value of iterate(
          input,
          { ...callOptions, signal: controller.signal },
          handlers.onOpen,
        )) {
          handlers.onData(value);
        }
        if (!controller.signal.aborted) {
          handlers.onDone?.();
        }
      } catch (error) {
        if (controller.signal.aborted) {
          return;
        }
        handlers.onError?.(
          error instanceof BowlineError
            ? error
            : new BowlineError("UNKNOWN", String(error), 0, { cause: error }),
        );
      }
    })();
    return () => controller.abort();
  };
  return iterate;
}

async function* transportEvents(
  transport: SubscriptionTransport,
  path: string,
  proc: ProcedureRuntime,
  contract: ContractRuntime,
  input: unknown,
  callOptions: CallOptions | undefined,
  onOpen: (() => void) | undefined,
): AsyncIterable<unknown> {
  for await (const event of transport.subscribe(path, input, callOptions?.signal)) {
    if (event.event === "open") {
      onOpen?.();
      continue;
    }
    if (event.event === "data") {
      yield proc.output === undefined
        ? event.payload
        : hydrate(event.payload, proc.output, contract.hydrators);
    } else if (event.event === "error") {
      throw errorFromEnvelope(0, event.payload, path, contract);
    } else {
      return;
    }
  }
}

async function* streamEvents(
  fetchFn: typeof fetch,
  base: string,
  path: string,
  proc: ProcedureRuntime,
  contract: ContractRuntime,
  headersSource: HeadersSource | undefined,
  input: unknown,
  callOptions: CallOptions | undefined,
  onOpen: (() => void) | undefined,
): AsyncIterable<unknown> {
  const { url, init } = await prepare(
    base,
    path,
    proc,
    headersSource,
    input,
    callOptions,
    "text/event-stream",
  );
  init.cache = "no-store";
  const response = await send(fetchFn, url, init, path, callOptions?.signal);
  if (!response.ok) {
    throw toError(response.status, await response.text(), path, contract);
  }
  if (!response.body) {
    throw new BowlineError("UNKNOWN", `empty stream from ${path}`, response.status);
  }
  onOpen?.();
  try {
    for await (const event of parseEventStream(response.body)) {
      if (event.event === "message") {
        const data = parseBody(event.data, 200, path);
        yield proc.output === undefined ? data : hydrate(data, proc.output, contract.hydrators);
      } else if (event.event === "error") {
        throw toError(response.status, event.data, path, contract);
      } else if (event.event === "done") {
        return;
      }
    }
  } catch (cause) {
    if (callOptions?.signal?.aborted && !(cause instanceof BowlineError)) {
      throw new BowlineError("CANCELED", "subscription aborted", 0, { cause });
    }
    throw cause;
  }
}

function parseBody(text: string, status: number, path: string): unknown {
  if (text === "") {
    return undefined;
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw new BowlineError("UNKNOWN", `HTTP ${status} from ${path} is not valid JSON`, status);
  }
}

function toError(
  status: number,
  text: string,
  path: string,
  contract: ContractRuntime,
): BowlineError {
  try {
    const parsed = JSON.parse(text) as { error?: unknown };
    return errorFromEnvelope(status, parsed.error, path, contract);
  } catch {}
  return new BowlineError("UNKNOWN", `HTTP ${status} from ${path}`, status);
}

function errorFromEnvelope(
  status: number,
  raw: unknown,
  path: string,
  contract: ContractRuntime,
): BowlineError {
  const e = raw as
    | { code?: string; message?: string; type?: string; details?: unknown; issues?: Issue[] }
    | undefined;
  if (e && typeof e.code === "string" && typeof e.message === "string") {
    const options: BowlineErrorOptions = {};
    if (typeof e.type === "string") {
      options.type = e.type;
    }
    if (e.details !== undefined) {
      options.details =
        typeof e.type === "string" ? hydrate(e.details, e.type, contract.hydrators) : e.details;
    }
    if (e.issues !== undefined) {
      options.issues = e.issues;
    }
    return new BowlineError(e.code as Code, e.message, status, options);
  }
  return new BowlineError("UNKNOWN", `malformed error from ${path}`, status);
}
