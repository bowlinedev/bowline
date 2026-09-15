import type { ContractField, ContractType } from "@bowline/client";
import type { ContractIndex, TypeEntry } from "./contract.js";

function names(index: ContractIndex): Map<string, string> {
  const counts = new Map<string, number>();
  for (const decl of index.types) {
    counts.set(decl.name, (counts.get(decl.name) ?? 0) + 1);
  }
  const out = new Map<string, string>();
  for (const decl of index.types) {
    if ((counts.get(decl.name) ?? 0) > 1) {
      const pkg = decl.id.slice(0, decl.id.lastIndexOf("."));
      const last = pkg.slice(pkg.lastIndexOf("/") + 1);
      out.set(decl.id, `${last.charAt(0).toUpperCase()}${last.slice(1)}_${decl.name}`);
    } else {
      out.set(decl.id, decl.name);
    }
  }
  return out;
}

function primitive(name: string, encoding?: string): string {
  switch (name) {
    case "string":
    case "bytes":
      return "string";
    case "bool":
      return "boolean";
    case "timestamp":
      return "Date";
    case "raw":
      return "unknown";
    case "int64":
    case "uint64":
      return encoding === "string" ? "bigint" : "number";
    default:
      return "number";
  }
}

export function typeText(
  index: ContractIndex,
  type: ContractType,
  byId: Map<string, string>,
): string {
  let text: string;
  switch (type.kind) {
    case "primitive":
      text = primitive(type.name ?? "string", type.encoding);
      break;
    case "ref": {
      const decl = type.id === undefined ? undefined : index.type(type.id);
      if (decl?.kind === "primitive") {
        text = primitive(decl.primitive ?? "string", type.encoding);
      } else {
        text = byId.get(type.id ?? "") ?? "unknown";
        if (type.args !== undefined && type.args.length > 0) {
          text += `<${type.args.map((a) => typeText(index, a, byId)).join(", ")}>`;
        }
      }
      break;
    }
    case "array": {
      const elem = type.elem === undefined ? "unknown" : typeText(index, type.elem, byId);
      text = elem.includes(" | ") ? `(${elem})[]` : `${elem}[]`;
      break;
    }
    case "map":
      text = `Record<string, ${type.value === undefined ? "unknown" : typeText(index, type.value, byId)}>`;
      break;
    case "struct":
      text = `{ ${(type.fields ?? []).map((f) => fieldText(index, f, byId)).join(" ")} }`;
      break;
    case "param":
      text = type.name ?? "T";
      break;
    default:
      text = "unknown";
  }
  return type.nullable === true ? `${text} | null` : text;
}

function fieldText(index: ContractIndex, field: ContractField, byId: Map<string, string>): string {
  const value = typeText(index, field.type, byId);
  return `${field.name}${field.optional ? "?" : ""}: ${field.nullable ? `${value} | null` : value};`;
}

export function declarationText(
  index: ContractIndex,
  decl: TypeEntry,
  byId: Map<string, string>,
): string {
  const name = byId.get(decl.id) ?? decl.name;
  const lines: string[] = [];
  if (decl.doc !== undefined && decl.doc !== "") {
    lines.push(`/** ${decl.doc} */`);
  }
  switch (decl.kind) {
    case "enum":
      lines.push(
        `export type ${name} = ${(decl.values ?? []).map((v) => JSON.stringify(v.value)).join(" | ")};`,
      );
      break;
    case "primitive":
      lines.push(`export type ${name} = ${primitive(decl.primitive ?? "string")};`);
      break;
    case "struct":
      lines.push(`export interface ${name} {`);
      for (const f of decl.fields ?? []) {
        lines.push(`  ${fieldText(index, f, byId)}`);
      }
      lines.push("}");
      break;
    case "generic":
      lines.push(`export interface ${name}<${(decl.params ?? []).join(", ")}> {`);
      for (const f of decl.body?.fields ?? []) {
        lines.push(`  ${fieldText(index, f, byId)}`);
      }
      lines.push("}");
      break;
    default:
      lines.push(`export type ${name} = unknown;`);
  }
  return lines.join("\n");
}

export function renderDeclarations(index: ContractIndex): string {
  const byId = names(index);
  return index.types.map((decl) => declarationText(index, decl, byId)).join("\n\n");
}

export function TypesPane({ index }: { index: ContractIndex }) {
  const byId = names(index);
  return (
    <div className="types">
      {index.types.map((decl) => (
        <pre key={decl.id} id={`type-${decl.id}`}>
          {declarationText(index, decl, byId)}
        </pre>
      ))}
    </div>
  );
}
