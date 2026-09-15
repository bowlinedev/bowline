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

import type {
  Client as ErrorsClient,
  ProcedureError,
} from "../../../cmd/bowline/internal/gen/ts/testdata/errors.golden.js";

declare const errorsClient: ErrorsClient;
declare const voidError: ProcedureError<"void">;

if (voidError.type === "InvoiceLocked") {
  expectTypeOf(voidError.details.since).toEqualTypeOf<Date>();
} else if (voidError.type === "QuotaExceeded") {
  expectTypeOf(voidError.details.limit).toEqualTypeOf<number>();
} else {
  expectTypeOf(voidError.type).toEqualTypeOf<undefined>();
}
expectTypeOf(errorsClient.void.safe).returns.resolves.toMatchTypeOf<{ ok: boolean }>();
