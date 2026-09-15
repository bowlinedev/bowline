import type { SubscriptionTransport, TransportEvent } from "./types.js";

interface Frame {
  id: number;
  type: "subscribe" | "stop" | "data" | "error" | "done";
  path?: string;
  input?: unknown;
  data?: unknown;
  error?: unknown;
}

export interface WebSocketTransportOptions {
  WebSocket?: typeof WebSocket;
  protocols?: string | string[];
}

class Queue<T> {
  private readonly items: T[] = [];
  private waiting: ((value: IteratorResult<T>) => void) | undefined;
  private closed = false;

  push(item: T): void {
    if (this.closed) {
      return;
    }
    if (this.waiting) {
      const resolve = this.waiting;
      this.waiting = undefined;
      resolve({ value: item, done: false });
      return;
    }
    this.items.push(item);
  }

  close(): void {
    this.closed = true;
    if (this.waiting) {
      const resolve = this.waiting;
      this.waiting = undefined;
      resolve({ value: undefined, done: true });
    }
  }

  next(): Promise<IteratorResult<T>> {
    const item = this.items.shift();
    if (item !== undefined) {
      return Promise.resolve({ value: item, done: false });
    }
    if (this.closed) {
      return Promise.resolve({ value: undefined, done: true });
    }
    return new Promise((resolve) => {
      this.waiting = resolve;
    });
  }
}

export function websocketTransport(
  url: string,
  options: WebSocketTransportOptions = {},
): SubscriptionTransport {
  const Impl = options.WebSocket ?? globalThis.WebSocket;
  let socket: WebSocket | undefined;
  let ready: Promise<WebSocket> | undefined;
  let nextId = 1;
  const queues = new Map<number, Queue<TransportEvent>>();

  const connect = (): Promise<WebSocket> => {
    if (ready) {
      return ready;
    }
    ready = new Promise((resolve, reject) => {
      const ws = options.protocols === undefined ? new Impl(url) : new Impl(url, options.protocols);
      ws.addEventListener("open", () => {
        socket = ws;
        resolve(ws);
      });
      ws.addEventListener("message", (event: MessageEvent) => {
        const frame = JSON.parse(String(event.data)) as Frame;
        const queue = queues.get(frame.id);
        if (!queue) {
          return;
        }
        if (frame.type === "data") {
          queue.push({ event: "data", payload: frame.data });
        } else if (frame.type === "error") {
          queue.push({ event: "error", payload: frame.error });
          queue.close();
          queues.delete(frame.id);
        } else if (frame.type === "done") {
          queue.push({ event: "done", payload: undefined });
          queue.close();
          queues.delete(frame.id);
        }
      });
      ws.addEventListener("close", () => {
        for (const [id, queue] of queues) {
          queue.push({
            event: "error",
            payload: { code: "UNAVAILABLE", message: "connection closed" },
          });
          queue.close();
          queues.delete(id);
        }
        socket = undefined;
        ready = undefined;
      });
      ws.addEventListener("error", () => {
        if (!socket) {
          ready = undefined;
          reject(new Error("websocket connection failed"));
        }
      });
    });
    return ready;
  };

  const send = (ws: WebSocket, frame: Frame) => ws.send(JSON.stringify(frame));

  return {
    async *subscribe(path, input, signal) {
      const ws = await connect();
      const id = nextId++;
      const queue = new Queue<TransportEvent>();
      queues.set(id, queue);
      const stop = () => {
        if (queues.delete(id)) {
          queue.close();
          if (ws.readyState === ws.OPEN) {
            send(ws, { id, type: "stop" });
          }
        }
      };
      signal?.addEventListener("abort", stop, { once: true });
      send(ws, { id, type: "subscribe", path, input: input ?? {} });
      yield { event: "open", payload: undefined };
      try {
        while (true) {
          const { value, done } = await queue.next();
          if (done) {
            return;
          }
          yield value;
          if (value.event !== "data") {
            return;
          }
        }
      } finally {
        signal?.removeEventListener("abort", stop);
        stop();
      }
    },
  };
}
