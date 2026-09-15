import { expectTypeOf } from "vitest";
import type {
  Client as Generics,
  Page,
  User,
} from "../../../cmd/bowline/internal/gen/ts/testdata/generics.golden.js";
import type { Client as Routing } from "../../../cmd/bowline/internal/gen/ts/testdata/routing.golden.js";
import { createClient } from "../../../cmd/bowline/internal/gen/ts/testdata/stdlib.golden.js";

declare const generics: Generics;
declare const routing: Routing;

expectTypeOf(generics.get).parameter(0).toEqualTypeOf<Page<User>>();
expectTypeOf(routing.admin.purge.kind).toEqualTypeOf<"mutation">();
expectTypeOf(routing.get.kind).toEqualTypeOf<"query">();

const stdlib = createClient({ url: "http://localhost" });
expectTypeOf(stdlib.get).returns.resolves.toHaveProperty("at").toEqualTypeOf<Date>();
expectTypeOf(stdlib.get).returns.resolves.toHaveProperty("big").toEqualTypeOf<bigint>();
