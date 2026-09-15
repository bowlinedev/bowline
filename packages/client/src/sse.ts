export interface ServerEvent {
  event: string;
  data: string;
}

export async function* parseEventStream(
  body: ReadableStream<Uint8Array>,
): AsyncIterable<ServerEvent> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let event = "message";
  let data: string[] = [];
  const flush = (): ServerEvent | undefined => {
    if (data.length === 0) {
      event = "message";
      return undefined;
    }
    const out = { event, data: data.join("\n") };
    event = "message";
    data = [];
    return out;
  };
  const handleLine = (line: string) => {
    if (line.startsWith(":")) {
      return;
    }
    const colon = line.indexOf(":");
    const field = colon < 0 ? line : line.slice(0, colon);
    let rest = colon < 0 ? "" : line.slice(colon + 1);
    if (rest.startsWith(" ")) {
      rest = rest.slice(1);
    }
    if (field === "event") {
      event = rest;
    } else if (field === "data") {
      data.push(rest);
    }
  };
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      buffer += decoder.decode(value, { stream: true });
      let index = buffer.search(/\r\n|\r|\n/);
      while (index >= 0) {
        const line = buffer.slice(0, index);
        const separator = buffer.startsWith("\r\n", index) ? 2 : 1;
        buffer = buffer.slice(index + separator);
        if (line === "") {
          const complete = flush();
          if (complete) {
            yield complete;
          }
        } else {
          handleLine(line);
        }
        index = buffer.search(/\r\n|\r|\n/);
      }
    }
    if (buffer !== "") {
      handleLine(buffer);
      buffer = "";
    }
    const trailing = flush();
    if (trailing) {
      yield trailing;
    }
  } finally {
    reader.releaseLock();
  }
}
