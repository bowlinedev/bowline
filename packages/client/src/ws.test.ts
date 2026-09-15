import { expect, test } from "vitest";
import { createClient } from "./client.js";
import type { BowlineError } from "./error.js";
import type { ContractRuntime, Subscription } from "./types.js";
import { websocketTransport } from "./ws.js";

class FakeSocket extends EventTarget {
  static instances: FakeSocket[] = [];
  readonly OPEN = 1;
  readonly sent: string[] = [];
  readyState = 0;
  constructor(public readonly url: string) {
    super();
    FakeSocket.instances.push(this);
    setTimeout(() => {
      this.readyState = 1;
      this.dispatchEvent(new Event("open"));
    }, 0);
  }
  send(data: string) {
    this.sent.push(data);
  }
  receive(frame: unknown) {
    this.dispatchEvent(new MessageEvent("message", { data: JSON.stringify(frame) }));
  }
  close() {
    this.readyState = 3;
    this.dispatchEvent(new Event("close"));
  }
}

const contract: ContractRuntime = {
  version: "1.0",
  hydrators: { Change: [{ path: ["at"], kind: "timestamp" }] },
  procedures: { "invoices.watch": { kind: "subscription", method: "GET", output: "Change" } },
};

interface Client {
  invoices: { watch: Subscription<{ limit: number }, { id: number; at: Date }> };
}

function setup() {
  FakeSocket.instances = [];
  const transport = websocketTransport("ws://api.test/ws", {
    WebSocket: FakeSocket as unknown as typeof WebSocket,
  });
  const client = createClient(contract, { url: "http://api.test", transport }) as Client;
  return { client };
}

async function tick() {
  await new Promise((r) => setTimeout(r, 5));
}

test("subscribes over a socket with allocated ids and hydrates data frames", async () => {
  const { client } = setup();
  const values: number[] = [];
  const done = (async () => {
    for await (const v of client.invoices.watch({ limit: 2 })) {
      values.push(v.id);
      expect(v.at).toBeInstanceOf(Date);
    }
  })();
  await tick();
  const socket = FakeSocket.instances[0] as FakeSocket;
  expect(JSON.parse(socket.sent[0] as string)).toEqual({
    id: 1,
    type: "subscribe",
    path: "invoices.watch",
    input: { limit: 2 },
  });
  socket.receive({ id: 1, type: "data", data: { id: 1, at: "2026-09-15T00:00:00Z" } });
  socket.receive({ id: 1, type: "data", data: { id: 2, at: "2026-09-16T00:00:00Z" } });
  socket.receive({ id: 1, type: "done" });
  await done;
  expect(values).toEqual([1, 2]);
  const second = client.invoices.watch({ limit: 1 })[Symbol.asyncIterator]();
  const pending = second.next();
  await tick();
  expect(JSON.parse(socket.sent[1] as string).id).toBe(2);
  socket.receive({ id: 2, type: "done" });
  await pending;
  expect(FakeSocket.instances).toHaveLength(1);
});

test("error frames reject with BowlineError and stop frames follow unsubscribe", async () => {
  const { client } = setup();
  const errors: BowlineError[] = [];
  const stop = client.invoices.watch.subscribe(
    { limit: 1 },
    { onData: () => {}, onError: (e) => errors.push(e) },
  );
  await tick();
  const socket = FakeSocket.instances[0] as FakeSocket;
  socket.receive({ id: 1, type: "error", error: { code: "NOT_FOUND", message: "gone" } });
  await tick();
  expect(errors[0]?.code).toBe("NOT_FOUND");
  stop();
  const second = client.invoices.watch.subscribe({ limit: 1 }, { onData: () => {} });
  await tick();
  second();
  await tick();
  expect(
    socket.sent.filter((s) => JSON.parse(s).type === "stop").map((s) => JSON.parse(s).id),
  ).toEqual([2]);
});

test("closing the socket fails every active subscription", async () => {
  const { client } = setup();
  const failed: string[] = [];
  client.invoices.watch.subscribe(
    { limit: 1 },
    { onData: () => {}, onError: (e) => failed.push(e.code) },
  );
  await tick();
  (FakeSocket.instances[0] as FakeSocket).close();
  await tick();
  expect(failed).toEqual(["UNAVAILABLE"]);
});
