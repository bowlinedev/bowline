import {
  type ClientOptions,
  type Interaction,
  type RecordSink,
  websocketTransport,
} from "@bowline/client";
import { bowlineQuery } from "@bowline/react-query";
import { createClient } from "./bowline.js";

const params = new URLSearchParams(window.location.search);
export const transportName = params.get("transport") === "ws" ? "ws" : "sse";

function socketUrl(): string {
  const scheme = window.location.protocol === "https:" ? "wss" : "ws";
  return `${scheme}://${window.location.host}/ws`;
}

declare global {
  interface Window {
    __bowlineInteractions?: Interaction[];
  }
}

function recorder(): RecordSink | undefined {
  let enabled = false;
  try {
    enabled = window.localStorage.getItem("bowline.record") === "1";
  } catch {
    enabled = false;
  }
  if (!enabled) {
    return undefined;
  }
  window.__bowlineInteractions = [];
  return {
    consumer: "ledger-web",
    write(interaction) {
      window.__bowlineInteractions?.push(interaction);
    },
  };
}

const record = recorder();

const options: ClientOptions = { url: "/api" };
if (transportName === "ws") {
  options.transport = websocketTransport(socketUrl());
}
if (record !== undefined) {
  options.record = record;
}

export const client = createClient(options);
export const bq = bowlineQuery(client);
