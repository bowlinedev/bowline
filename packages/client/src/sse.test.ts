import { expect, test } from "vitest";
import { parseEventStream } from "./sse.js";

function streamOf(chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  return new ReadableStream({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk));
      }
      controller.close();
    },
  });
}

async function collect(chunks: string[]) {
  const out = [];
  for await (const e of parseEventStream(streamOf(chunks))) {
    out.push(e);
  }
  return out;
}

test("parses events split across chunk boundaries", async () => {
  const events = await collect([
    "event: mess",
    "age\nda",
    'ta: {"n":1}\n\nevent: done\ndata: {}\n\n',
  ]);
  expect(events).toEqual([
    { event: "message", data: '{"n":1}' },
    { event: "done", data: "{}" },
  ]);
});

test("joins multi-line data and ignores comments", async () => {
  const events = await collect([": ping\n\ndata: a\ndata: b\n\n"]);
  expect(events).toEqual([{ event: "message", data: "a\nb" }]);
});

test("accepts CRLF and a missing trailing blank line", async () => {
  const events = await collect(['event: error\r\ndata: {"x":1}\r\n\r\ndata: tail']);
  expect(events).toEqual([
    { event: "error", data: '{"x":1}' },
    { event: "message", data: "tail" },
  ]);
});
