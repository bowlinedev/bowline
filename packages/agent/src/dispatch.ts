import {
  type BowlineError,
  type ClientOptions,
  type Result as ClientResult,
  type Code,
  type ContractDocument,
  type ContractRuntime,
  createClient,
  type HeadersSource,
  type Issue,
} from "@bowlinedev/client";
import type { Tool } from "./tools.js";

export interface Call {
  id: string;
  tool: string;
  input: unknown;
}

export interface ResultError {
  code: Code;
  message: string;
  type?: string;
  details?: unknown;
  issues?: Issue[];
}

export interface Result {
  output?: unknown;
  error?: ResultError;
}

export interface Tracer {
  start(call: Call): (result: Result) => void;
}

export interface DispatcherOptions {
  url: string;
  headers?: HeadersSource;
  fetch?: typeof fetch;
  tracer?: Tracer;
}

export interface Dispatcher {
  dispatch(call: Call): Promise<Result>;
}

type Leaf = {
  safe(input?: unknown): Promise<ClientResult<unknown, BowlineError>>;
};

export function createDispatcher(
  contract: ContractDocument,
  list: Tool[],
  options: DispatcherOptions,
): Dispatcher {
  const runtime: ContractRuntime = { version: contract.bowline, hydrators: {}, procedures: {} };
  const byName = new Map<string, Tool>();
  for (const tool of list) {
    const proc = contract.procedures.find((p) => p.path === tool.procedure);
    if (proc === undefined) {
      throw new Error(`tool ${tool.name} refers to unknown procedure ${tool.procedure}`);
    }
    runtime.procedures[proc.path] = { kind: proc.kind, method: proc.method };
    byName.set(tool.name, tool);
  }
  const clientOptions: ClientOptions = { url: options.url };
  if (options.headers !== undefined) {
    clientOptions.headers = options.headers;
  }
  if (options.fetch !== undefined) {
    clientOptions.fetch = options.fetch;
  }
  const client = createClient(runtime, clientOptions);
  return {
    async dispatch(call: Call): Promise<Result> {
      const finish = options.tracer?.start(call) ?? (() => {});
      const tool = byName.get(call.tool);
      if (tool === undefined) {
        const result: Result = {
          error: { code: "NOT_FOUND", message: `unknown tool ${call.tool}` },
        };
        finish(result);
        return result;
      }
      const leaf = walk(client, tool.procedure);
      const outcome = await leaf.safe(call.input);
      const result: Result = outcome.ok
        ? { output: outcome.value }
        : { error: toResultError(outcome.error) };
      finish(result);
      return result;
    },
  };
}

function walk(client: unknown, path: string): Leaf {
  let node = client;
  for (const segment of path.split(".")) {
    node = (node as Record<string, unknown>)[segment];
  }
  return node as Leaf;
}

function toResultError(error: BowlineError): ResultError {
  const out: ResultError = { code: error.code, message: error.message };
  if (error.type !== undefined) {
    out.type = error.type;
  }
  if (error.details !== undefined) {
    out.details = error.details;
  }
  if (error.issues.length > 0) {
    out.issues = error.issues;
  }
  return out;
}

export function recordingTracer(write: (line: string) => void): Tracer {
  return {
    start(call: Call) {
      const started = performance.now();
      return (result: Result) => {
        write(
          JSON.stringify({
            id: call.id,
            tool: call.tool,
            input: call.input ?? {},
            output: result.output ?? null,
            error: result.error ?? null,
            durationMs: Math.round(performance.now() - started),
          }),
        );
      };
    },
  };
}
