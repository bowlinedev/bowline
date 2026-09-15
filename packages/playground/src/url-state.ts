export interface ViewState {
  procedure?: string;
  input?: unknown;
  headers?: Record<string, string>;
}

function encodeBase64(text: string): string {
  const bytes = new TextEncoder().encode(text);
  let binary = "";
  for (const b of bytes) {
    binary += String.fromCharCode(b);
  }
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

function decodeBase64(encoded: string): string {
  const padded = encoded.replaceAll("-", "+").replaceAll("_", "/");
  const binary = atob(padded + "=".repeat((4 - (padded.length % 4)) % 4));
  const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

export function stripAuthorization(headers: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [name, value] of Object.entries(headers)) {
    if (name.toLowerCase() !== "authorization") {
      out[name] = value;
    }
  }
  return out;
}

export function encodeState(state: ViewState): string {
  const safe: ViewState = {};
  if (state.procedure !== undefined) {
    safe.procedure = state.procedure;
  }
  if (state.input !== undefined) {
    safe.input = state.input;
  }
  if (state.headers !== undefined) {
    const headers = stripAuthorization(state.headers);
    if (Object.keys(headers).length > 0) {
      safe.headers = headers;
    }
  }
  return encodeBase64(JSON.stringify(safe));
}

export function decodeState(fragment: string): ViewState | undefined {
  const raw = fragment.startsWith("#") ? fragment.slice(1) : fragment;
  if (raw === "") {
    return undefined;
  }
  try {
    const parsed: unknown = JSON.parse(decodeBase64(raw));
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
      return undefined;
    }
    const state = parsed as Record<string, unknown>;
    const out: ViewState = {};
    if (typeof state.procedure === "string") {
      out.procedure = state.procedure;
    }
    if (state.input !== undefined) {
      out.input = state.input;
    }
    if (typeof state.headers === "object" && state.headers !== null) {
      const headers: Record<string, string> = {};
      for (const [name, value] of Object.entries(state.headers as Record<string, unknown>)) {
        if (typeof value === "string") {
          headers[name] = value;
        }
      }
      out.headers = stripAuthorization(headers);
    }
    return out;
  } catch {
    return undefined;
  }
}
