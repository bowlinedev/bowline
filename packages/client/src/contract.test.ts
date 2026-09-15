import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { ContractDocument } from "./contract.js";

describe("ContractDocument", () => {
  it("types the committed ledger contract", () => {
    const doc = JSON.parse(
      readFileSync(
        new URL("../../../examples/ledger/api/bowline.contract.json", import.meta.url),
        "utf8",
      ),
    ) as ContractDocument;
    expect(doc.bowline).toBe("1.1");
    const get = doc.procedures.find((p) => p.path === "invoices.get");
    expect(get?.kind).toBe("query");
    expect(get?.tool?.scopes).toEqual(["billing"]);
    expect(get?.schemas?.input.$schema).toBe("https://json-schema.org/draft/2020-12/schema");
    expect(Object.keys(doc.errors)).toContain(
      "github.com/bowlinedev/bowline/examples/ledger/api.InvoiceLocked",
    );
  });
});
