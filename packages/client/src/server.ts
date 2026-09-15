import type { ClientOptions, HeadersSource } from "./types.js";

type HeaderBag = Headers | Record<string, string | string[] | undefined>;

export interface ServerClientOptions extends ClientOptions {
  forwardHeaders?: string[];
  request?: Request | { headers: HeaderBag };
  cache?: RequestCache;
  next?: { revalidate?: number | false; tags?: string[] };
}

const defaultForwarded = ["cookie", "authorization"];

export function createServerClient<C>(
  create: (options: ClientOptions) => C,
  options: ServerClientOptions,
): C {
  const { forwardHeaders, request, cache, next, headers, fetch: baseFetch, ...rest } = options;
  const forwarded = pickHeaders(request, forwardHeaders ?? defaultForwarded);
  const clientOptions: ClientOptions = { ...rest };
  if (headers !== undefined || [...forwarded.keys()].length > 0) {
    clientOptions.headers = mergeHeaders(forwarded, headers);
  }
  const underlying = baseFetch ?? globalThis.fetch;
  if (cache !== undefined || next !== undefined) {
    clientOptions.fetch = ((input: string | URL | Request, init?: RequestInit) => {
      const extended: RequestInit & { next?: ServerClientOptions["next"] } = { ...init };
      if (cache !== undefined) {
        extended.cache = cache;
      }
      if (next !== undefined) {
        extended.next = next;
      }
      return underlying(input, extended);
    }) as typeof fetch;
  } else if (baseFetch !== undefined) {
    clientOptions.fetch = baseFetch;
  }
  return create(clientOptions);
}

function pickHeaders(request: ServerClientOptions["request"], names: string[]): Headers {
  const out = new Headers();
  if (request === undefined) {
    return out;
  }
  const bag = request.headers;
  for (const name of names) {
    const value = readHeader(bag, name);
    if (value !== undefined) {
      out.set(name, value);
    }
  }
  return out;
}

function readHeader(bag: HeaderBag, name: string): string | undefined {
  if (bag instanceof Headers) {
    return bag.get(name) ?? undefined;
  }
  const lower = name.toLowerCase();
  for (const [key, value] of Object.entries(bag)) {
    if (key.toLowerCase() !== lower || value === undefined) {
      continue;
    }
    return Array.isArray(value) ? value.join(", ") : value;
  }
  return undefined;
}

function mergeHeaders(forwarded: Headers, source: HeadersSource | undefined): HeadersSource {
  return async () => {
    const merged = new Headers(forwarded);
    const extra = typeof source === "function" ? await source() : source;
    for (const [key, value] of new Headers(extra)) {
      merged.set(key, value);
    }
    return merged;
  };
}
