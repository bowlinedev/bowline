import { expect, test } from "vitest";
import { BowlineError } from "./error.js";

test("carries code, status, details, and issues", () => {
  const cause = new Error("socket");
  const err = new BowlineError("INVALID_ARGUMENT", "invalid input", 400, {
    details: { n: 1 },
    issues: [{ path: ["email"], rule: "email", message: "must be a valid email address" }],
    cause,
  });
  expect(err).toBeInstanceOf(Error);
  expect(err.name).toBe("BowlineError");
  expect(err.code).toBe("INVALID_ARGUMENT");
  expect(err.status).toBe(400);
  expect(err.details).toEqual({ n: 1 });
  expect(err.issues[0]?.path).toEqual(["email"]);
  expect(err.cause).toBe(cause);
  expect(String(err)).toBe("BowlineError: invalid input");
});

test("defaults issues to an empty array", () => {
  expect(new BowlineError("NOT_FOUND", "gone", 404).issues).toEqual([]);
});
