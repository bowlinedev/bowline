import type { ContractDocument, ContractProcedure } from "@bowline/client";

export interface Tool {
  name: string;
  procedure: string;
  description: string;
  inputSchema: Record<string, unknown>;
  outputSchema: Record<string, unknown>;
  readOnly: boolean;
  destructive: boolean;
  scopes: string[];
}

export interface ToolsOptions {
  scopes?: string[];
  readOnly?: boolean;
}

export function tools(contract: ContractDocument, options: ToolsOptions = {}): Tool[] {
  const out: Tool[] = [];
  const seen = new Map<string, string>();
  for (const proc of contract.procedures) {
    const tool = proc.tool;
    if (tool === undefined) {
      continue;
    }
    const readOnly = tool.readOnly === true;
    const scopes = tool.scopes ?? [];
    if (options.readOnly === true && !readOnly) {
      continue;
    }
    if (options.scopes !== undefined && options.scopes.length > 0) {
      if (!options.scopes.some((s) => scopes.includes(s))) {
        continue;
      }
    }
    const name = proc.path.replaceAll(".", "_");
    const other = seen.get(name);
    if (other !== undefined) {
      throw new Error(`tool name "${name}" is used by both ${other} and ${proc.path}`);
    }
    seen.set(name, proc.path);
    if (proc.schemas === undefined) {
      throw new Error(
        `procedure ${proc.path} has no embedded schemas; run bowline gen with "schemas": true`,
      );
    }
    out.push({
      name,
      procedure: proc.path,
      description: describe(contract, proc),
      inputSchema: proc.schemas.input,
      outputSchema: proc.schemas.output,
      readOnly,
      destructive: tool.destructive === true,
      scopes: [...scopes],
    });
  }
  return out;
}

export function describe(contract: ContractDocument, proc: ContractProcedure): string {
  let description = (proc.doc ?? "").trim();
  if (proc.errors !== undefined && proc.errors.length > 0) {
    const names: string[] = [];
    for (const id of proc.errors) {
      const decl = contract.errors[id];
      if (decl !== undefined) {
        names.push(decl.name);
      }
    }
    if (description !== "") {
      description += "\n";
    }
    description += `Errors: ${names.join(", ")}`;
  }
  return description;
}
