import { expectTypeOf } from "vitest";
import { schemas as generics } from "../../../cmd/bowline/internal/gen/ts/testdata/generics.zod.golden.js";
import type {
  Address,
  Person,
} from "../../../cmd/bowline/internal/gen/ts/testdata/structs.golden.js";
import {
  inputs,
  schemas,
} from "../../../cmd/bowline/internal/gen/ts/testdata/structs.zod.golden.js";

expectTypeOf(schemas.Address.parse({})).toEqualTypeOf<Address>();
expectTypeOf(schemas.Person.parse({})).toMatchTypeOf<Person>();
expectTypeOf(inputs.get.parse({})).toEqualTypeOf<Address>();
expectTypeOf(generics.Page(generics.User).parse({}).items[0]?.name).toEqualTypeOf<
  string | undefined
>();
expectTypeOf(generics.Tree(generics.User).parse({}).children[0]?.value.name).toEqualTypeOf<
  string | undefined
>();
