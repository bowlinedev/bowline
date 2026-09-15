import { describe, expect, it } from "vitest";
import type { ImpactReport } from "./api.js";
import { breaking, byConsumer, changeText, verdict } from "./impact.js";

const removedField = {
  category: "breaking",
  path: "procedure invoices.create output field total",
  message: "field removed",
};

const report: ImpactReport = {
  baseline: "sha256:abc",
  changes: [
    removedField,
    { category: "widened", path: "procedure invoices.list", message: "added" },
  ],
  affected: [
    { consumer: "ledger-web", change: removedField, reason: "reads total" },
    {
      consumer: "ledger-web",
      change: { ...removedField, path: "procedure invoices.get output field total" },
      reason: "reads total",
    },
    { consumer: "billing", via: "edge", change: removedField, reason: "calls invoices.create" },
  ],
  unattributed: [{ category: "breaking", path: "procedure admin.purge", message: "removed" }],
  ok: false,
};

describe("byConsumer", () => {
  it("groups the hits per consumer and keeps the gateway a separate row", () => {
    const groups = byConsumer(report);
    expect(groups.map((g) => [g.consumer, g.via, g.hits.length])).toEqual([
      ["ledger-web", undefined, 2],
      ["billing", "edge", 1],
    ]);
  });

  it("returns nothing for a clean report", () => {
    expect(byConsumer({ ...report, affected: [] })).toEqual([]);
  });
});

describe("verdict", () => {
  it("counts the breaks and the distinct consumers", () => {
    expect(verdict(report)).toBe("3 break(s) across 2 consumer(s).");
  });

  it("says when nothing is affected", () => {
    expect(verdict({ ...report, affected: [], ok: true })).toBe(
      "2 change(s); no known consumer breaks.",
    );
  });

  it("says when there are no changes at all", () => {
    expect(verdict({ ...report, changes: [], affected: [] })).toBe("No contract changes.");
  });

  it("says when nothing is tagged main", () => {
    expect(verdict({ ...report, baseline: "" })).toContain("nothing to compare");
  });
});

describe("changeText and breaking", () => {
  it("renders a change as its path and message", () => {
    expect(changeText(removedField)).toBe(
      "procedure invoices.create output field total: field removed",
    );
  });

  it("keeps only the categories the registry attributes", () => {
    expect(
      breaking([
        removedField,
        { category: "narrowed", path: "p", message: "m" },
        { category: "widened", path: "q", message: "n" },
      ]).map((c) => c.category),
    ).toEqual(["breaking", "narrowed"]);
  });
});
