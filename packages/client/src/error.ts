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
  details?: unknown;
  issues?: Issue[];
  cause?: unknown;
}

export class BowlineError extends Error {
  readonly code: Code;
  readonly status: number;
  readonly details: unknown;
  readonly issues: Issue[];

  constructor(code: Code, message: string, status: number, options: BowlineErrorOptions = {}) {
    super(message, options.cause === undefined ? undefined : { cause: options.cause });
    this.name = "BowlineError";
    this.code = code;
    this.status = status;
    this.details = options.details;
    this.issues = options.issues ?? [];
  }
}
