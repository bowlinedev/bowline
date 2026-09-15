import type { Tool } from "./tools.js";

export interface AnthropicTool {
  name: string;
  description: string;
  input_schema: Record<string, unknown>;
}

export interface OpenAITool {
  type: "function";
  function: {
    name: string;
    description: string;
    parameters: Record<string, unknown>;
  };
}

export interface SchemaTool {
  name: string;
  procedure: string;
  description: string;
  inputSchema: Record<string, unknown>;
  outputSchema: Record<string, unknown>;
  readOnly: boolean;
  destructive: boolean;
  scopes: string[];
}

export function toAnthropic(list: Tool[]): AnthropicTool[] {
  return list.map((t) => ({
    name: t.name,
    description: t.description,
    input_schema: t.inputSchema,
  }));
}

export function toOpenAI(list: Tool[]): OpenAITool[] {
  return list.map((t) => ({
    type: "function",
    function: { name: t.name, description: t.description, parameters: t.inputSchema },
  }));
}

export function toJSONSchema(list: Tool[]): SchemaTool[] {
  return list.map((t) => ({
    name: t.name,
    procedure: t.procedure,
    description: t.description,
    inputSchema: t.inputSchema,
    outputSchema: t.outputSchema,
    readOnly: t.readOnly,
    destructive: t.destructive,
    scopes: [...t.scopes].sort(),
  }));
}
