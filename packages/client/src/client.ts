import {
  BowlineError,
  type BowlineErrorOptions,
  type Code,
  type Issue,
  type Result,
} from "./error.js";
import { hydrate, serialize } from "./hydrate.js";
import type {
  CallOptions,
  ClientOptions,
  ContractRuntime,
  HeadersSource,
  ProcedureRuntime,
} from "./types.js";

type Leaf = ((input?: unknown, options?: CallOptions) => Promise<unknown>) & {
  kind: "query" | "mutation";
  safe: (input?: unknown, options?: CallOptions) => Promise<Result<unknown, BowlineError>>;
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
    const leaf = ((input?: unknown, callOptions?: CallOptions) =>
      call(fetchFn, base, path, proc, contract, options.headers, input, callOptions)) as Leaf;
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
    node[segments[segments.length - 1] as string] = leaf;
  }
  return root;
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
): Promise<unknown> {
  const headers = new Headers(
    typeof headersSource === "function" ? await headersSource() : headersSource,
  );
  for (const [k, v] of new Headers(callOptions?.headers)) {
    headers.set(k, v);
  }
  headers.set("accept", "application/json");
  const body = serialize(input ?? {});
  let url = `${base}/${path}`;
  const init: RequestInit = { method: proc.method, headers, signal: callOptions?.signal ?? null };
  if (proc.method === "GET") {
    if (input !== undefined) {
      url += `?input=${encodeURIComponent(body)}`;
    }
  } else {
    headers.set("content-type", "application/json");
    init.body = body;
  }
  let response: Response;
  try {
    response = await fetchFn(url, init);
  } catch (cause) {
    if (callOptions?.signal?.aborted) {
      throw new BowlineError("CANCELED", "request aborted", 0, { cause });
    }
    throw new BowlineError("UNAVAILABLE", `network error calling ${path}`, 0, { cause });
  }
  const text = await response.text();
  if (!response.ok) {
    throw toError(response.status, text, path, contract);
  }
  const data: unknown = text === "" ? undefined : JSON.parse(text);
  return proc.output === undefined ? data : hydrate(data, proc.output, contract.hydrators);
}

function toError(
  status: number,
  text: string,
  path: string,
  contract: ContractRuntime,
): BowlineError {
  try {
    const parsed = JSON.parse(text) as {
      error?: {
        code?: string;
        message?: string;
        type?: string;
        details?: unknown;
        issues?: Issue[];
      };
    };
    const e = parsed.error;
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
  } catch {}
  return new BowlineError("UNKNOWN", `HTTP ${status} from ${path}`, status);
}
