import { readFileSync } from "node:fs";
import type { ContractDocument } from "@bowline/client";
import { describe, expect, expectTypeOf, it } from "vitest";
import { toAnthropic, toJSONSchema, toOpenAI } from "./encode.js";
import { type Tool, tools } from "./tools.js";

const root = new URL("../../../", import.meta.url);

function ledger(): ContractDocument {
  return JSON.parse(
    readFileSync(new URL("examples/ledger/api/bowline.contract.json", root), "utf8"),
  ) as ContractDocument;
}

function golden(format: string): unknown {
  return JSON.parse(
    readFileSync(
      new URL(`cmd/bowline/internal/tools/testdata/ledger.${format}.golden.json`, root),
      "utf8",
    ),
  );
}

describe("tools", () => {
  it("lists exposed procedures from the ledger contract", () => {
    const list = tools(ledger());
    expect(list.map((t) => t.name)).toEqual([
      "customers_search",
      "invoices_get",
      "invoices_list",
      "invoices_void",
    ]);
    expect(list[3]?.destructive).toBe(true);
    expect(list[3]?.description).toContain("Errors: InvoiceLocked");
    expectTypeOf<Tool["inputSchema"]>().toEqualTypeOf<Record<string, unknown>>();
  });

  it("filters by scope intersection and read-only hint", () => {
    expect(tools(ledger(), { scopes: ["crm"] }).map((t) => t.name)).toEqual(["customers_search"]);
    expect(tools(ledger(), { readOnly: true }).map((t) => t.name)).toEqual([
      "customers_search",
      "invoices_get",
      "invoices_list",
    ]);
    expect(tools(ledger(), { scopes: ["nope"] })).toEqual([]);
  });

  it("rejects contracts without embedded schemas", () => {
    const doc = ledger();
    for (const proc of doc.procedures) {
      delete proc.schemas;
    }
    expect(() => tools(doc)).toThrow(/schemas/);
  });

  it("matches the CLI goldens for every format", () => {
    const list = tools(ledger());
    expect(toAnthropic(list)).toEqual(golden("anthropic"));
    expect(toOpenAI(list)).toEqual(golden("openai"));
    expect(toJSONSchema(list)).toEqual(golden("json-schema"));
  });
});
