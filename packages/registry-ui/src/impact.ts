import type { Affected, Change, ImpactReport } from "./api.js";

export interface ConsumerImpact {
  consumer: string;
  via?: string;
  hits: Affected[];
}

export function byConsumer(report: ImpactReport): ConsumerImpact[] {
  const out: ConsumerImpact[] = [];
  const index = new Map<string, ConsumerImpact>();
  for (const hit of report.affected) {
    const key = JSON.stringify([hit.consumer, hit.via ?? ""]);
    let group = index.get(key);
    if (group === undefined) {
      group =
        hit.via === undefined
          ? { consumer: hit.consumer, hits: [] }
          : { consumer: hit.consumer, via: hit.via, hits: [] };
      index.set(key, group);
      out.push(group);
    }
    group.hits.push(hit);
  }
  return out;
}

export function changeText(change: Change): string {
  return `${change.path}: ${change.message}`;
}

export function verdict(report: ImpactReport): string {
  if (report.baseline === "") {
    return "No baseline is tagged main; there is nothing to compare against.";
  }
  if (report.changes.length === 0) {
    return "No contract changes.";
  }
  if (report.affected.length === 0) {
    return `${report.changes.length} change(s); no known consumer breaks.`;
  }
  const consumers = new Set(report.affected.map((hit) => hit.consumer));
  return `${report.affected.length} break(s) across ${consumers.size} consumer(s).`;
}

export function breaking(changes: Change[]): Change[] {
  return changes.filter((c) => c.category === "breaking" || c.category === "narrowed");
}
