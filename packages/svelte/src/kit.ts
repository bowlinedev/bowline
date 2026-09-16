import type { ClientOptions, HeadersSource } from "@bowlinedev/client";
import type { RequestEvent } from "@sveltejs/kit";

const forwarded = ["cookie", "authorization"];

export function serverClient<C>(
  event: RequestEvent,
  create: (options: ClientOptions) => C,
  options: ClientOptions,
): C {
  const base = options.headers;
  const headers: HeadersSource = async () => {
    const merged = new Headers(typeof base === "function" ? await base() : base);
    for (const name of forwarded) {
      const value = event.request.headers.get(name);
      if (value !== null) {
        merged.set(name, value);
      }
    }
    return merged;
  };
  return create({ ...options, fetch: event.fetch, headers });
}
