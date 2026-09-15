import { describe, expect, it } from "vitest";
import { decodeState, encodeState } from "./url-state.js";

describe("url state", () => {
  it("round trips procedure, input, and headers", () => {
    const encoded = encodeState({
      procedure: "invoices.get",
      input: { id: 3, note: "ünïcödé" },
      headers: { "X-Tenant": "acme" },
    });
    expect(encoded).toMatch(/^[A-Za-z0-9_-]+$/);
    expect(decodeState(`#${encoded}`)).toEqual({
      procedure: "invoices.get",
      input: { id: 3, note: "ünïcödé" },
      headers: { "X-Tenant": "acme" },
    });
  });

  it("never encodes the Authorization header", () => {
    const encoded = encodeState({
      procedure: "invoices.get",
      headers: { Authorization: "Bearer secret", authorization: "x", "X-Tenant": "acme" },
    });
    expect(atob(encoded.replaceAll("-", "+").replaceAll("_", "/"))).not.toContain("secret");
    expect(decodeState(encoded)?.headers).toEqual({ "X-Tenant": "acme" });
    const only = encodeState({ procedure: "p", headers: { Authorization: "Bearer secret" } });
    expect(decodeState(only)).toEqual({ procedure: "p" });
  });

  it("ignores garbage fragments", () => {
    expect(decodeState("")).toBeUndefined();
    expect(decodeState("#not-base64-json")).toBeUndefined();
    expect(decodeState(`#${btoa("[1]")}`)).toBeUndefined();
  });
});
