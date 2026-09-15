import type { Graph, GraphEdge, GraphNode, NodeKind } from "./api.js";

export interface PlacedNode extends GraphNode {
  x: number;
  y: number;
  column: number;
}

export interface PlacedEdge extends GraphEdge {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

export interface Layout {
  nodes: PlacedNode[];
  edges: PlacedEdge[];
  width: number;
  height: number;
}

export const nodeWidth = 168;
export const nodeHeight = 40;

const columnGap = 110;
const rowGap = 24;
const margin = 20;

const columns: NodeKind[] = ["consumer", "gateway", "service"];

function columnOf(kind: NodeKind): number {
  const index = columns.indexOf(kind);
  return index === -1 ? columns.length - 1 : index;
}

export function layout(graph: Graph): Layout {
  const sorted = [...graph.nodes].sort((a, b) => a.name.localeCompare(b.name));
  const lanes = columns.map((_, column) => sorted.filter((node) => columnOf(node.kind) === column));
  const used = lanes.filter((lane) => lane.length > 0);
  const rows = Math.max(1, ...used.map((lane) => lane.length));
  const height = margin * 2 + rows * nodeHeight + (rows - 1) * rowGap;
  const width = margin * 2 + used.length * nodeWidth + Math.max(0, used.length - 1) * columnGap;

  const placed = new Map<string, PlacedNode>();
  let column = 0;
  for (const lane of lanes) {
    if (lane.length === 0) {
      continue;
    }
    const span = lane.length * nodeHeight + (lane.length - 1) * rowGap;
    const top = (height - span) / 2;
    for (const [row, node] of lane.entries()) {
      placed.set(node.name, {
        ...node,
        column,
        x: margin + column * (nodeWidth + columnGap),
        y: top + row * (nodeHeight + rowGap),
      });
    }
    column += 1;
  }

  const edges: PlacedEdge[] = [];
  for (const edge of graph.edges) {
    const from = placed.get(edge.from);
    const to = placed.get(edge.to);
    if (from === undefined || to === undefined) {
      continue;
    }
    const forward = from.column <= to.column;
    edges.push({
      ...edge,
      x1: forward ? from.x + nodeWidth : from.x,
      y1: from.y + nodeHeight / 2,
      x2: forward ? to.x : to.x + nodeWidth,
      y2: to.y + nodeHeight / 2,
    });
  }
  return { nodes: [...placed.values()], edges, width, height };
}
