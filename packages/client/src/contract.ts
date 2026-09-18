export type TypeKind =
  | "primitive"
  | "ref"
  | "array"
  | "map"
  | "struct"
  | "enum"
  | "generic"
  | "param";

export interface ContractRule {
  rule: string;
  param?: string;
}

export interface ContractType {
  kind: TypeKind;
  name?: string;
  id?: string;
  encoding?: string;
  nullable?: boolean;
  elem?: ContractType;
  key?: ContractType;
  value?: ContractType;
  length?: number;
  fields?: ContractField[];
  args?: ContractType[];
}

export interface ContractField {
  name: string;
  type: ContractType;
  doc?: string;
  optional?: boolean;
  nullable?: boolean;
  rules?: ContractRule[];
  example?: unknown;
}

export interface ContractEnumValue {
  name: string;
  value: string | number;
}

export interface ContractTypeDecl {
  kind: TypeKind;
  name: string;
  doc?: string;
  primitive?: string;
  base?: string;
  origin?: string;
  params?: string[];
  fields?: ContractField[];
  values?: ContractEnumValue[];
  body?: ContractType;
}

export interface ContractErrorDecl {
  name: string;
  code: string;
  doc?: string;
  fields?: ContractField[];
}

export interface ContractTool {
  scopes?: string[];
  readOnly?: boolean;
  destructive?: boolean;
}

export interface ContractSchemas {
  input: Record<string, unknown>;
  output: Record<string, unknown>;
}

export interface ContractProcedure {
  path: string;
  kind: "query" | "mutation" | "subscription" | "upload";
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  httpPath?: string;
  doc?: string;
  deprecated?: string;
  idempotent?: boolean;
  input: ContractType;
  output: ContractType;
  goInput?: string;
  goOutput?: string;
  errors?: string[];
  meta?: Record<string, string>;
  tool?: ContractTool;
  schemas?: ContractSchemas;
}

export interface ContractPosition {
  file: string;
  line: number;
}

export interface ContractDocument {
  bowline: string;
  hash?: string;
  types: Record<string, ContractTypeDecl>;
  errors: Record<string, ContractErrorDecl>;
  procedures: ContractProcedure[];
  positions?: Record<string, ContractPosition>;
}
