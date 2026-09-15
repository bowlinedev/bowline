import type { APIRoute } from "astro";

const upstream = "http://localhost:8080/api";
const forwarded = ["content-type", "accept", "authorization", "cookie", "idempotency-key"];
const returned = ["content-type", "deprecation", "sunset", "allow", "idempotent-replayed"];

export const ALL: APIRoute = async ({ params, request, url }) => {
  const headers = new Headers();
  for (const name of forwarded) {
    const value = request.headers.get(name);
    if (value !== null) {
      headers.set(name, value);
    }
  }
  const target = `${upstream}/${params.path ?? ""}${url.search}`;
  const init: RequestInit = { method: request.method, headers };
  if (request.method !== "GET" && request.method !== "HEAD") {
    init.body = await request.arrayBuffer();
  }
  const response = await fetch(target, init);
  const out = new Headers();
  for (const name of returned) {
    const value = response.headers.get(name);
    if (value !== null) {
      out.set(name, value);
    }
  }
  return new Response(await response.arrayBuffer(), { status: response.status, headers: out });
};
