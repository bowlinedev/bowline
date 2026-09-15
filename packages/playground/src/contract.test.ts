import { readFileSync } from "node:fs";
import type { ContractDocument } from "@bowline/client";
import { describe, expect, it } from "vitest";
import { indexContract, shortName, typeLabel } from "./contract.js";

const ledger = JSON.parse(
  readFileSync(
    new URL("../../../examples/ledger/api/bowline.contract.json", import.meta.url),
    "utf8",
  ),
) as ContractDocument;

describe("indexContract", () => {
  it("sorts procedures and groups them by path prefix", () => {
    const index = indexContract(ledger);
    const paths = index.procedures.map((p) => p.path);
    expect(paths).toEqual([...paths].sort());
    expect(paths).toContain("invoices.get");
    const invoices = index.tree.find((n) => n.name === "invoices");
    expect(invoices?.procedure).toBeUndefined();
    expect(invoices?.children.map((c) => c.path)).toContain("invoices.get");
    expect(index.tree.find((n) => n.name === "health")?.procedure?.kind).toBe("query");
  });

  it("looks up types and procedures by id and path", () => {
    const index = indexContract(ledger);
    const get = index.procedure("invoices.get");
    expect(get?.method).toBe("GET");
    const output = get?.output;
    expect(output?.kind).toBe("ref");
    expect(index.type(output?.id ?? "")?.name).toBe("Invoice");
    expect(index.procedure("nope")).toBeUndefined();
    expect(index.types.map((t) => t.name)).toContain("Customer");
  });

  it("labels types the way the generated client names them", () => {
    const index = indexContract(ledger);
    const search = index.procedure("customers.search");
    expect(typeLabel(index, search?.output as never)).toBe("Page<Customer>");
    expect(typeLabel(index, { kind: "array", elem: { kind: "primitive", name: "string" } })).toBe(
      "string[]",
    );
    expect(shortName("github.com/acme/app.User")).toBe("User");
  });
});
