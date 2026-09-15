import type { HydrateEntry, HydrateKind } from "./types.js";

export function hydrate(
  value: unknown,
  key: string,
  hydrators: Record<string, HydrateEntry[]>,
): unknown {
  const entries = hydrators[key];
  if (!entries || value === null || value === undefined) {
    return value;
  }
  for (const entry of entries) {
    walk(value, entry.path, 0, entry.kind, hydrators);
  }
  return value;
}

function walk(
  node: unknown,
  path: string[],
  index: number,
  kind: HydrateKind,
  hydrators: Record<string, HydrateEntry[]>,
): void {
  if (node === null || node === undefined) {
    return;
  }
  if (index === path.length) {
    return;
  }
  const segment = path[index];
  if (segment === undefined) {
    return;
  }
  const last = index === path.length - 1;
  if (segment === "*") {
    if (Array.isArray(node)) {
      for (let i = 0; i < node.length; i++) {
        if (last) {
          node[i] = convert(node[i], kind, hydrators);
        } else {
          walk(node[i], path, index + 1, kind, hydrators);
        }
      }
    } else if (typeof node === "object") {
      const record = node as Record<string, unknown>;
      for (const k of Object.keys(record)) {
        if (last) {
          record[k] = convert(record[k], kind, hydrators);
        } else {
          walk(record[k], path, index + 1, kind, hydrators);
        }
      }
    }
    return;
  }
  if (typeof node !== "object" || Array.isArray(node)) {
    return;
  }
  const record = node as Record<string, unknown>;
  if (!(segment in record)) {
    return;
  }
  if (last) {
    record[segment] = convert(record[segment], kind, hydrators);
  } else {
    walk(record[segment], path, index + 1, kind, hydrators);
  }
}

function convert(
  value: unknown,
  kind: HydrateKind,
  hydrators: Record<string, HydrateEntry[]>,
): unknown {
  if (value === null || value === undefined) {
    return value;
  }
  if (kind === "timestamp") {
    return typeof value === "string" ? new Date(value) : value;
  }
  if (kind === "bigint") {
    return typeof value === "string" || typeof value === "number" ? BigInt(value) : value;
  }
  return hydrate(value, kind.ref, hydrators);
}

export function serialize(input: unknown): string {
  return JSON.stringify(input, (_key, value: unknown) =>
    typeof value === "bigint" ? value.toString() : value,
  );
}
