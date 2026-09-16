import { readFileSync } from "node:fs";
import type { ContractDocument } from "@bowlinedev/client";
import { describe, expect, it } from "vitest";
import { indexContract } from "./contract.js";
import { enumOptions, initialValue } from "./form.js";
import { renderDeclarations } from "./types-pane.js";

function row(name: string): ContractDocument {
  return JSON.parse(
    readFileSync(
      new URL(
        `../../../cmd/bowline/internal/analyzer/testdata/fidelity/rows/${name}/expected.contract.json`,
        import.meta.url,
      ),
      "utf8",
    ),
  ) as ContractDocument;
}

describe("initialValue", () => {
  it("uses examples for every node kind and defaults for the rest", () => {
    const index = indexContract(row("examples"));
    const get = index.procedure("get");
    const value = initialValue(index, get?.output as never) as Record<string, unknown>;
    expect(value).toMatchObject({
      name: "Ada Lovelace",
      email: "ada@example.com",
      age: 36,
      big: "9007199254740993",
      ratio: 0.75,
      active: true,
      status: "sent",
      level: 10,
      slug: "ada-lovelace",
      since: "2026-01-01T00:00:00Z",
      timeout: 1500000000,
      blob: "aGVsbG8=",
      raw: { any: 1 },
      tags: ["a", "b"],
      counts: { x: 1 },
      home: { city: "Rome" },
      nick: "ada",
      ratings: [],
    });
  });

  it("fills required fields, skips optional ones, and nulls nullable ones", () => {
    const index = indexContract(row("pointers"));
    for (const procedure of index.procedures) {
      const value = initialValue(index, procedure.output);
      expect(value).not.toBeUndefined();
    }
    const ledger = indexContract(
      JSON.parse(
        readFileSync(
          new URL("../../../examples/ledger/api/bowline.contract.json", import.meta.url),
          "utf8",
        ),
      ) as ContractDocument,
    );
    const create = ledger.procedure("invoices.create");
    const input = initialValue(ledger, create?.input as never) as Record<string, unknown>;
    expect(input.customerId).toBe(0);
    expect(Array.isArray(input.lines)).toBe(true);
    expect((input.lines as unknown[]).length).toBe(1);
    expect(input).not.toHaveProperty("note");
    const list = initialValue(ledger, ledger.procedure("invoices.list")?.input as never) as Record<
      string,
      unknown
    >;
    expect(list.limit).toBe(1);
    expect(list).not.toHaveProperty("cursor");
  });

  it("caps recursion and offers enum and oneof options", () => {
    const recursive = indexContract(row("recursive"));
    for (const procedure of recursive.procedures) {
      expect(() => JSON.stringify(initialValue(recursive, procedure.output))).not.toThrow();
    }
    const enums = indexContract(row("enums"));
    const order = enums.procedure("get")?.output as never;
    const value = initialValue(enums, order) as Record<string, unknown>;
    expect(value.status).toBe("draft");
    expect(value.level).toBe(1);
    const decl = enums.types.find((t) => t.name === "Order");
    const status = decl?.fields?.find((f) => f.name === "status");
    expect(enumOptions(enums, status?.type as never, status)).toEqual(["draft", "sent"]);
  });
});

describe("renderDeclarations", () => {
  it("renders declarations as TypeScript", () => {
    const text = renderDeclarations(indexContract(row("examples")));
    expect(text).toContain('export type Status = "draft" | "sent";');
    expect(text).toContain("export type Level = 1 | 10;");
    expect(text).toContain("export interface Sample {");
    expect(text).toContain("big: bigint;");
    expect(text).toContain("since: Date;");
    expect(text).toContain("nick?: string;");
    expect(text).toContain("ratings: Level[];");
    expect(text).toContain("counts: Record<string, number>;");
    const generics = renderDeclarations(indexContract(row("generics")));
    expect(generics).toContain("export interface Page<T> {");
    expect(generics).toContain("items: T[];");
    expect(generics).toContain("Page<Page<User>>");
  });
});
