import { expect, test } from "vitest";
import { version } from "./index.js";

test("exports a version", () => {
  expect(version).toBe("1.1.0");
});
