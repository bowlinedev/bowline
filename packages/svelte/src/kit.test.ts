import type { ClientOptions } from "@bowlinedev/client";
import type { RequestEvent } from "@sveltejs/kit";
import { describe, expect, it, vi } from "vitest";
import { serverClient } from "./kit.js";

function event(): RequestEvent {
  return {
    fetch: vi.fn(),
    request: new Request("http://x", {
      headers: { cookie: "s=1", authorization: "Bearer t", "x-other": "no" },
    }),
  } as unknown as RequestEvent;
}

async function resolve(options: ClientOptions): Promise<Headers> {
  const source = options.headers;
  return new Headers(typeof source === "function" ? await source() : source);
}

describe("serverClient", () => {
  it("uses event.fetch and forwards cookie and authorization only", async () => {
    const e = event();
    let received: ClientOptions | undefined;
    const client = serverClient(
      e,
      (options) => {
        received = options;
        return { url: options.url };
      },
      { url: "http://api/api", headers: { "x-app": "ledger" } },
    );
    expect(client).toEqual({ url: "http://api/api" });
    expect(received?.fetch).toBe(e.fetch);
    const headers = await resolve(received as ClientOptions);
    expect(headers.get("cookie")).toBe("s=1");
    expect(headers.get("authorization")).toBe("Bearer t");
    expect(headers.get("x-app")).toBe("ledger");
    expect(headers.get("x-other")).toBeNull();
  });

  it("resolves function header sources lazily", async () => {
    let received: ClientOptions | undefined;
    serverClient(
      event(),
      (options) => {
        received = options;
        return options;
      },
      { url: "/api", headers: async () => ({ "x-app": "lazy" }) },
    );
    const headers = await resolve(received as ClientOptions);
    expect(headers.get("x-app")).toBe("lazy");
    expect(headers.get("cookie")).toBe("s=1");
  });
});
