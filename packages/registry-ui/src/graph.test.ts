import { describe, expect, it } from "vitest";
import type { Graph } from "./api.js";
import { layout, nodeHeight, nodeWidth } from "./graph.js";

const federation: Graph = {
  nodes: [
    { name: "billing", kind: "service" },
    { name: "edge", kind: "gateway" },
    { name: "ledger", kind: "service" },
    { name: "ledger-web", kind: "consumer" },
  ],
  edges: [
    { from: "edge", to: "billing", kind: "composes" },
    { from: "edge", to: "ledger", kind: "composes" },
    { from: "ledger-web", to: "ledger", kind: "consumes" },
  ],
};

describe("layout", () => {
  it("puts consumers left of gateways and gateways left of services", () => {
    const placed = layout(federation);
    const at = (name: string) => placed.nodes.find((n) => n.name === name);
    expect(at("ledger-web")?.column).toBe(0);
    expect(at("edge")?.column).toBe(1);
    expect(at("ledger")?.column).toBe(2);
    expect(at("billing")?.column).toBe(2);
    expect(at("billing")?.x).toBeGreaterThan(at("edge")?.x ?? 0);
  });

  it("stacks a column without overlapping and centres it", () => {
    const placed = layout(federation);
    const services = placed.nodes.filter((n) => n.kind === "service").sort((a, b) => a.y - b.y);
    expect(services.map((n) => n.name)).toEqual(["billing", "ledger"]);
    const top = services[0]?.y ?? 0;
    const bottom = services[1]?.y ?? 0;
    expect(bottom - top).toBeGreaterThanOrEqual(nodeHeight);
    expect(placed.height - (bottom + nodeHeight - top)).toBeCloseTo(2 * top, 5);
  });

  it("draws every edge from the right of the source to the left of the target", () => {
    const placed = layout(federation);
    expect(placed.edges).toHaveLength(3);
    for (const edge of placed.edges) {
      const from = placed.nodes.find((n) => n.name === edge.from);
      const to = placed.nodes.find((n) => n.name === edge.to);
      expect(edge.x1).toBe((from?.x ?? 0) + nodeWidth);
      expect(edge.x2).toBe(to?.x);
      expect(edge.y1).toBe((from?.y ?? 0) + nodeHeight / 2);
    }
  });

  it("drops an edge whose endpoint is not a node", () => {
    const placed = layout({
      nodes: [{ name: "ledger", kind: "service" }],
      edges: [{ from: "ghost", to: "ledger", kind: "consumes" }],
    });
    expect(placed.edges).toEqual([]);
  });

  it("collapses empty columns so a single lane starts at the margin", () => {
    const placed = layout({
      nodes: [
        { name: "a", kind: "service" },
        { name: "b", kind: "service" },
      ],
      edges: [],
    });
    expect(placed.nodes.every((n) => n.column === 0)).toBe(true);
    expect(placed.width).toBe(nodeWidth + 40);
  });
});
