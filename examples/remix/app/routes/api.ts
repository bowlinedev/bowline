import { backend } from "../../src/ledger.js";
import type { Route } from "./+types/api";

const forwarded = ["accept", "authorization", "content-type", "cookie", "idempotency-key"];

async function proxy(request: Request, splat: string | undefined): Promise<Response> {
  const incoming = new URL(request.url);
  const target = new URL(`${backend}/${splat ?? ""}`);
  target.search = incoming.search;
  const headers = new Headers();
  for (const name of forwarded) {
    const value = request.headers.get(name);
    if (value !== null) {
      headers.set(name, value);
    }
  }
  const init: RequestInit & { duplex?: "half" } = { method: request.method, headers };
  if (request.method !== "GET" && request.method !== "HEAD") {
    init.body = request.body;
    init.duplex = "half";
  }
  const upstream = await fetch(target, init);
  const out = new Headers();
  for (const name of ["content-type", "deprecation", "allow", "idempotent-replayed"]) {
    const value = upstream.headers.get(name);
    if (value !== null) {
      out.set(name, value);
    }
  }
  return new Response(upstream.body, { status: upstream.status, headers: out });
}

export function loader({ request, params }: Route.LoaderArgs) {
  return proxy(request, params["*"]);
}

export function action({ request, params }: Route.ActionArgs) {
  return proxy(request, params["*"]);
}
