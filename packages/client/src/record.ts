export interface Interaction {
  procedure: string;
  method?: "GET" | "POST";
  input: unknown;
  response: { status: number; body: unknown };
}

export interface RecordSink {
  consumer: string;
  write(interaction: Interaction): void;
}

export function canonicalInput(input: unknown): string {
  return JSON.stringify(canonicalize(input ?? {}));
}

function canonicalize(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(canonicalize);
  }
  if (typeof value === "bigint") {
    return value.toString();
  }
  if (value instanceof Date) {
    return value.toISOString();
  }
  if (value !== null && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const key of Object.keys(value as Record<string, unknown>).sort()) {
      const v = (value as Record<string, unknown>)[key];
      if (v !== undefined) {
        out[key] = canonicalize(v);
      }
    }
    return out;
  }
  return value;
}

export function record(
  sink: RecordSink | undefined,
  procedure: string,
  method: "GET" | "POST",
  input: unknown,
  status: number,
  text: string,
): void {
  if (sink === undefined) {
    return;
  }
  let body: unknown = null;
  if (text !== "") {
    try {
      body = JSON.parse(text);
    } catch {
      body = text;
    }
  }
  sink.write({
    procedure,
    method,
    input: canonicalize(input ?? {}),
    response: { status, body },
  });
}
