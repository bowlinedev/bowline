import { describe, expect, it } from "vitest";
import { HISTORY_KEY, HISTORY_LIMIT, type HistoryEntry, load, push, save } from "./history.js";

function entry(i: number): HistoryEntry {
  return {
    at: new Date(i * 1000).toISOString(),
    procedure: "invoices.get",
    input: { id: i },
    status: 200,
    durationMs: i,
    response: { id: i },
  };
}

class MemoryStorage {
  data = new Map<string, string>();
  getItem(key: string): string | null {
    return this.data.get(key) ?? null;
  }
  setItem(key: string, value: string): void {
    this.data.set(key, value);
  }
}

describe("history", () => {
  it("keeps the newest entry first and caps at the limit", () => {
    let list: HistoryEntry[] = [];
    for (let i = 0; i < HISTORY_LIMIT + 10; i++) {
      list = push(list, entry(i));
    }
    expect(list).toHaveLength(HISTORY_LIMIT);
    expect(list[0]?.durationMs).toBe(HISTORY_LIMIT + 9);
    expect(list[HISTORY_LIMIT - 1]?.durationMs).toBe(10);
  });

  it("saves and loads through storage and tolerates bad data", () => {
    const store = new MemoryStorage();
    save([entry(1), entry(2)], store);
    expect(load(store).map((e) => e.durationMs)).toEqual([1, 2]);
    store.setItem(HISTORY_KEY, "not json");
    expect(load(store)).toEqual([]);
    expect(load(undefined)).toEqual([]);
  });
});
