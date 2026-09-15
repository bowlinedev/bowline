import { websocketTransport } from "@bowline/client";
import { bowlineQuery } from "@bowline/react-query";
import { createClient } from "./bowline.js";

const params = new URLSearchParams(window.location.search);
export const transportName = params.get("transport") === "ws" ? "ws" : "sse";

function socketUrl(): string {
  const scheme = window.location.protocol === "https:" ? "wss" : "ws";
  return `${scheme}://${window.location.host}/ws`;
}

export const client = createClient(
  transportName === "ws"
    ? { url: "/api", transport: websocketTransport(socketUrl()) }
    : { url: "/api" },
);
export const bq = bowlineQuery(client);
