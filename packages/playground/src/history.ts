export interface HistoryEntry {
  at: string;
  procedure: string;
  input: unknown;
  status: number;
  durationMs: number;
  response: unknown;
}

export const HISTORY_LIMIT = 50;
export const HISTORY_KEY = "bowline.playground.history";
export const HEADERS_KEY = "bowline.playground.headers";

export function push(list: HistoryEntry[], entry: HistoryEntry): HistoryEntry[] {
  return [entry, ...list].slice(0, HISTORY_LIMIT);
}

interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

function storage(): StorageLike | undefined {
  try {
    return globalThis.localStorage;
  } catch {
    return undefined;
  }
}

export function load(store: StorageLike | undefined = storage()): HistoryEntry[] {
  try {
    const raw = store?.getItem(HISTORY_KEY);
    if (raw === null || raw === undefined) {
      return [];
    }
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as HistoryEntry[]).slice(0, HISTORY_LIMIT) : [];
  } catch {
    return [];
  }
}

export function save(list: HistoryEntry[], store: StorageLike | undefined = storage()): void {
  try {
    store?.setItem(HISTORY_KEY, JSON.stringify(list.slice(0, HISTORY_LIMIT)));
  } catch {}
}

export function loadHeaders(store: StorageLike | undefined = storage()): Record<string, string> {
  try {
    const raw = store?.getItem(HEADERS_KEY);
    if (raw === null || raw === undefined) {
      return {};
    }
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null) {
      return {};
    }
    const out: Record<string, string> = {};
    for (const [name, value] of Object.entries(parsed as Record<string, unknown>)) {
      if (typeof value === "string") {
        out[name] = value;
      }
    }
    return out;
  } catch {
    return {};
  }
}

export function saveHeaders(
  headers: Record<string, string>,
  store: StorageLike | undefined = storage(),
): void {
  try {
    store?.setItem(HEADERS_KEY, JSON.stringify(headers));
  } catch {}
}
