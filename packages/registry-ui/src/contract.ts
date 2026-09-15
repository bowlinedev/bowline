import type {
  ContractDocument,
  ContractProcedure,
  ContractType,
  ContractTypeDecl,
} from "@bowline/client";

export interface TreeNode {
  name: string;
  path: string;
  procedure?: ContractProcedure;
  children: TreeNode[];
}

export type TypeEntry = ContractTypeDecl & { id: string };

export interface ContractIndex {
  doc: ContractDocument;
  procedures: ContractProcedure[];
  tree: TreeNode[];
  types: TypeEntry[];
  type(id: string): ContractTypeDecl | undefined;
  procedure(path: string): ContractProcedure | undefined;
}

export function indexContract(doc: ContractDocument): ContractIndex {
  const procedures = [...doc.procedures].sort((a, b) => a.path.localeCompare(b.path));
  const byPath = new Map(procedures.map((p) => [p.path, p]));
  const types = Object.entries(doc.types)
    .map(([id, decl]) => ({ ...decl, id }))
    .sort((a, b) => a.name.localeCompare(b.name) || a.id.localeCompare(b.id));
  return {
    doc,
    procedures,
    tree: buildTree(procedures),
    types,
    type: (id) => doc.types[id],
    procedure: (path) => byPath.get(path),
  };
}

export function buildTree(procedures: ContractProcedure[]): TreeNode[] {
  const roots: TreeNode[] = [];
  for (const procedure of procedures) {
    const segments = procedure.path.split(".");
    let level = roots;
    let prefix = "";
    for (const [i, segment] of segments.entries()) {
      prefix = prefix === "" ? segment : `${prefix}.${segment}`;
      const last = i === segments.length - 1;
      let node = level.find((n) => n.name === segment && (last ? n.procedure : !n.procedure));
      if (node === undefined) {
        node = last
          ? { name: segment, path: prefix, procedure, children: [] }
          : { name: segment, path: prefix, children: [] };
        level.push(node);
      }
      level = node.children;
    }
  }
  return roots;
}

export function shortName(id: string): string {
  const dot = id.lastIndexOf(".");
  return dot === -1 ? id : id.slice(dot + 1);
}

export function typeLabel(index: ContractIndex, type: ContractType): string {
  switch (type.kind) {
    case "primitive":
      return type.name ?? "unknown";
    case "ref": {
      const decl = type.id === undefined ? undefined : index.type(type.id);
      const base = decl?.name ?? shortName(type.id ?? "");
      if (type.args !== undefined && type.args.length > 0) {
        return `${base}<${type.args.map((a) => typeLabel(index, a)).join(", ")}>`;
      }
      return base;
    }
    case "array":
      return `${type.elem === undefined ? "unknown" : typeLabel(index, type.elem)}[]`;
    case "map":
      return `Record<${type.key === undefined ? "string" : typeLabel(index, type.key)}, ${type.value === undefined ? "unknown" : typeLabel(index, type.value)}>`;
    case "struct":
      return "object";
    case "param":
      return type.name ?? "T";
    default:
      return type.kind;
  }
}
