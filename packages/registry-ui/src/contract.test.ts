import type { ContractDocument } from "@bowlinedev/client";
import { describe, expect, it } from "vitest";
import { indexContract, shortName, typeLabel } from "./contract.js";
import { renderDeclarations } from "./types-pane.js";

const doc: ContractDocument = {
  bowline: "1.2",
  hash: "sha256:fixture",
  types: {
    "example.com/ledger.Invoice": {
      kind: "struct",
      name: "Invoice",
      fields: [
        { name: "id", type: { kind: "primitive", name: "int64" } },
        { name: "total", type: { kind: "primitive", name: "string" } },
        { name: "paidAt", type: { kind: "primitive", name: "timestamp" }, optional: true },
      ],
    },
    "example.com/ledger.Status": {
      kind: "enum",
      name: "Status",
      values: [
        { name: "Open", value: "open" },
        { name: "Paid", value: "paid" },
      ],
    },
    "example.com/ledger.Page": {
      kind: "generic",
      name: "Page",
      params: ["T"],
      body: {
        kind: "struct",
        fields: [{ name: "items", type: { kind: "array", elem: { kind: "param", name: "T" } } }],
      },
    },
  },
  errors: {},
  procedures: [
    {
      path: "invoices.list",
      kind: "query",
      method: "GET",
      input: { kind: "struct", fields: [] },
      output: {
        kind: "ref",
        id: "example.com/ledger.Page",
        args: [{ kind: "ref", id: "example.com/ledger.Invoice" }],
      },
    },
    {
      path: "invoices.get",
      kind: "query",
      method: "GET",
      input: { kind: "struct", fields: [] },
      output: { kind: "ref", id: "example.com/ledger.Invoice" },
    },
    {
      path: "health",
      kind: "query",
      method: "GET",
      input: { kind: "struct", fields: [] },
      output: { kind: "primitive", name: "bool" },
    },
  ],
};

describe("indexContract", () => {
  it("sorts procedures and groups them by path prefix", () => {
    const index = indexContract(doc);
    expect(index.procedures.map((p) => p.path)).toEqual([
      "health",
      "invoices.get",
      "invoices.list",
    ]);
    const invoices = index.tree.find((n) => n.name === "invoices");
    expect(invoices?.procedure).toBeUndefined();
    expect(invoices?.children.map((c) => c.path)).toEqual(["invoices.get", "invoices.list"]);
    expect(index.tree.find((n) => n.name === "health")?.procedure?.kind).toBe("query");
  });

  it("looks up types and procedures", () => {
    const index = indexContract(doc);
    expect(index.procedure("invoices.get")?.method).toBe("GET");
    expect(index.procedure("nope")).toBeUndefined();
    expect(index.type("example.com/ledger.Invoice")?.name).toBe("Invoice");
    expect(index.types.map((t) => t.name)).toEqual(["Invoice", "Page", "Status"]);
    expect(shortName("example.com/ledger.Invoice")).toBe("Invoice");
    expect(shortName("Invoice")).toBe("Invoice");
  });

  it("labels a generic instantiation the way the generated client names it", () => {
    const index = indexContract(doc);
    const list = index.procedure("invoices.list");
    expect(typeLabel(index, list?.output as never)).toBe("Page<Invoice>");
    expect(typeLabel(index, { kind: "array", elem: { kind: "primitive", name: "string" } })).toBe(
      "string[]",
    );
  });
});

describe("renderDeclarations", () => {
  it("renders every declaration the contract browser shows", () => {
    const text = renderDeclarations(indexContract(doc));
    expect(text).toContain("export interface Invoice {");
    expect(text).toContain("  id: number;");
    expect(text).toContain("  paidAt?: Date;");
    expect(text).toContain('export type Status = "open" | "paid";');
    expect(text).toContain("export interface Page<T> {");
    expect(text).toContain("  items: T[];");
  });
});
