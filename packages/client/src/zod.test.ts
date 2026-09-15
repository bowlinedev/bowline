import { expect, test } from "vitest";
import {
  inputs,
  schemas,
} from "../../../cmd/bowline/internal/gen/ts/testdata/structs.zod.golden.js";

test("generated schemas enforce validation rules at runtime", () => {
  const valid = {
    name: "Ada",
    age: 36,
    email: "ada@example.com",
    home: { street: "1 Row", city: "London" },
    NoTag: "",
    inline: { x: 1 },
  };
  expect(schemas.Person.safeParse(valid).success).toBe(true);
  const invalid = schemas.Person.safeParse({ ...valid, email: "not-an-email" });
  expect(invalid.success).toBe(false);
  expect(schemas.Address.safeParse({ street: "x", city: "" }).success).toBe(false);
  expect(inputs.get.safeParse({ street: "x", city: "Paris" }).success).toBe(true);
});
