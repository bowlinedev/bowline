import { describe, expect, it } from "vitest";
import { href, parse, type Route } from "./route.js";

describe("parse", () => {
  it("reads every view out of the hash", () => {
    expect(parse("")).toEqual({ view: "services" });
    expect(parse("#/")).toEqual({ view: "services" });
    expect(parse("#/graph")).toEqual({ view: "graph" });
    expect(parse("#/impact")).toEqual({ view: "impact" });
    expect(parse("#/impact/ledger")).toEqual({ view: "impact", name: "ledger" });
    expect(parse("#/services/ledger")).toEqual({ view: "service", name: "ledger" });
    expect(parse("#/services/ledger/versions/sha256:abc")).toEqual({
      view: "contract",
      name: "ledger",
      hash: "sha256:abc",
    });
  });

  it("falls back to the service list for anything it does not know", () => {
    expect(parse("#/nope/deeper")).toEqual({ view: "services" });
    expect(parse("#/services")).toEqual({ view: "services" });
  });

  it("decodes names that need escaping", () => {
    expect(parse("#/services/a%2Fb")).toEqual({ view: "service", name: "a/b" });
  });
});

describe("href", () => {
  it("round-trips every route", () => {
    const routes: Route[] = [
      { view: "services" },
      { view: "graph" },
      { view: "impact" },
      { view: "impact", name: "ledger" },
      { view: "service", name: "a/b" },
      { view: "contract", name: "ledger", hash: "sha256:abc" },
    ];
    for (const route of routes) {
      expect(parse(href(route))).toEqual(route);
    }
  });
});
