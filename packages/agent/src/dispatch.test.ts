import { readFileSync } from "node:fs";
import type { ContractDocument } from "@bowline/client";
import { describe, expect, it } from "vitest";
import { type Call, createDispatcher, type Result, recordingTracer } from "./dispatch.js";
import { tools } from "./tools.js";

const contract = JSON.parse(
  readFileSync(
    new URL("../../../examples/ledger/api/bowline.contract.json", import.meta.url),
    "utf8",
  ),
) as ContractDocument;

function fakeFetch(handler: (url: string, init: RequestInit) => Response): typeof fetch {
  return (async (input: string | URL | Request, init?: RequestInit) =>
    handler(String(input), init ?? {})) as typeof fetch;
}

describe("createDispatcher", () => {
  it("dispatches queries as GET with the input parameter and forwards headers", async () => {
    const seen: { url: string; headers: Headers }[] = [];
    const dispatcher = createDispatcher(contract, tools(contract), {
      url: "http://ledger/api/",
      headers: { authorization: "Bearer dev" },
      fetch: fakeFetch((url, init) => {
        seen.push({ url, headers: new Headers(init.headers) });
        return new Response(JSON.stringify({ id: 3, status: "sent" }), { status: 200 });
      }),
    });
    const result = await dispatcher.dispatch({ id: "1", tool: "invoices_get", input: { id: 3 } });
    expect(result).toEqual({ output: { id: 3, status: "sent" } });
    expect(seen[0]?.url).toBe(
      `http://ledger/api/invoices.get?input=${encodeURIComponent('{"id":3}')}`,
    );
    expect(seen[0]?.headers.get("authorization")).toBe("Bearer dev");
  });

  it("maps error envelopes and network failures to result errors", async () => {
    const dispatcher = createDispatcher(contract, tools(contract), {
      url: "http://ledger/api",
      fetch: fakeFetch((url) => {
        if (url.includes("invoices.void")) {
          throw new TypeError("connection refused");
        }
        return new Response(
          JSON.stringify({
            error: {
              code: "NOT_FOUND",
              message: "invoice 999 not found",
              issues: [{ path: ["id"], rule: "exists", message: "missing" }],
            },
          }),
          { status: 404 },
        );
      }),
    });
    const notFound = await dispatcher.dispatch({
      id: "1",
      tool: "invoices_get",
      input: { id: 999 },
    });
    expect(notFound.error?.code).toBe("NOT_FOUND");
    expect(notFound.error?.message).toBe("invoice 999 not found");
    expect(notFound.error?.issues).toHaveLength(1);
    const down = await dispatcher.dispatch({ id: "2", tool: "invoices_void", input: { id: 3 } });
    expect(down.error?.code).toBe("UNAVAILABLE");
    const unknown = await dispatcher.dispatch({ id: "3", tool: "nope", input: {} });
    expect(unknown.error?.code).toBe("NOT_FOUND");
  });

  it("calls the tracer around every dispatch and records JSON lines", async () => {
    const events: string[] = [];
    const lines: string[] = [];
    const inner = recordingTracer((line) => lines.push(line));
    const dispatcher = createDispatcher(contract, tools(contract), {
      url: "http://ledger/api",
      fetch: fakeFetch(() => new Response("[]", { status: 200 })),
      tracer: {
        start(call: Call) {
          events.push(`start ${call.id}`);
          const finish = inner.start(call);
          return (result: Result) => {
            events.push(`finish ${call.id} ${result.error === undefined ? "ok" : "error"}`);
            finish(result);
          };
        },
      },
    });
    await dispatcher.dispatch({ id: "a", tool: "invoices_list", input: { limit: 2 } });
    expect(events).toEqual(["start a", "finish a ok"]);
    const record = JSON.parse(lines[0] ?? "{}") as Record<string, unknown>;
    expect(record.id).toBe("a");
    expect(record.tool).toBe("invoices_list");
    expect(record.input).toEqual({ limit: 2 });
    expect(record.output).toEqual([]);
    expect(record.error).toBeNull();
    expect(typeof record.durationMs).toBe("number");
  });
});
