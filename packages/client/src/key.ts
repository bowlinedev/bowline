export type ProcedureKey<I> = readonly [readonly string[]] | readonly [readonly string[], I];

export function procedureKey<I>(path: readonly string[], input?: I): ProcedureKey<I> {
  return input === undefined ? [path] : [path, input];
}

export type ProcedureBrand = {
  readonly kind: "query" | "mutation" | "subscription" | "upload";
  readonly types?: { readonly input: unknown; readonly output: unknown };
};

export type InputOf<T> = T extends { readonly types?: { readonly input: infer I } } ? I : never;

export type OutputOf<T> = T extends { readonly types?: { readonly output: infer O } } ? O : never;

export type ProcedureFn = ((input?: unknown, options?: unknown) => Promise<unknown>) &
  ProcedureBrand;

export function walkProcedures(
  node: Record<string, unknown>,
  path: readonly string[],
  visit: (fn: ProcedureFn, path: readonly string[]) => unknown,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(node)) {
    const next = [...path, key];
    if (typeof value === "function" && "kind" in value) {
      const built = visit(value as ProcedureFn, next);
      if (built !== undefined) {
        out[key] = built;
      }
    } else if (value && typeof value === "object") {
      out[key] = walkProcedures(value as Record<string, unknown>, next, visit);
    }
  }
  return out;
}
