export type Code =
  | "CANCELED"
  | "UNKNOWN"
  | "INVALID_ARGUMENT"
  | "DEADLINE_EXCEEDED"
  | "NOT_FOUND"
  | "ALREADY_EXISTS"
  | "PERMISSION_DENIED"
  | "RESOURCE_EXHAUSTED"
  | "FAILED_PRECONDITION"
  | "ABORTED"
  | "OUT_OF_RANGE"
  | "UNIMPLEMENTED"
  | "INTERNAL"
  | "UNAVAILABLE"
  | "DATA_LOSS"
  | "UNAUTHENTICATED";

export interface Issue {
  path: string[];
  rule: string;
  message: string;
}

export interface BowlineErrorOptions {
  type?: string;
  details?: unknown;
  issues?: Issue[];
  cause?: unknown;
}

export class BowlineError extends Error {
  readonly code: Code;
  readonly status: number;
  readonly type: string | undefined;
  readonly details: unknown;
  readonly issues: Issue[];

  constructor(code: Code, message: string, status: number, options: BowlineErrorOptions = {}) {
    super(message, options.cause === undefined ? undefined : { cause: options.cause });
    this.name = "BowlineError";
    this.code = code;
    this.status = status;
    this.type = options.type;
    this.details = options.details;
    this.issues = options.issues ?? [];
  }
}

export type TypedError<T extends string, D> = BowlineError & {
  readonly type: T;
  readonly details: D;
};

export type Result<O, E> = { ok: true; value: O } | { ok: false; error: E };

export type UntypedError = BowlineError & { readonly type: undefined };
