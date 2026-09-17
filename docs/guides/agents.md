# Agent SDKs

There are three small packages that read a contract, produce provider-specific tool definitions, and forward tool calls to a running Bowline API. They do not interpret types themselves. The schemas embedded in the contract are the source of truth, so the definitions a model sees are the same as what `bowline export tools` prints. All three require a contract generated with `"schemas": true`.

## Go

Module `github.com/bowlinedev/bowline/agent`. Standard library only.

sketch: a tour of the module's API in one block; error handling is elided so the sequence stays readable

```go
doc, err := agent.Load("api/bowline.contract.json")
tools, err := agent.Tools(doc, agent.Scopes("billing"))
definitions, err := agent.Encode(tools, agent.Anthropic)

caller := agent.HTTPCaller("http://localhost:8080/api", http.Header{"Authorization": {"Bearer dev"}}, nil)
dispatcher := agent.NewDispatcher(tools, caller, agent.WithTracer(agent.NewRecordingTracer(os.Stdout)))

result, err := dispatcher.Dispatch(ctx, agent.Call{ID: "1", Tool: "invoices_get", Input: json.RawMessage(`{"id":3}`)})
if result.Error != nil {
	switch result.Error.Code {
	case bowline.NotFound:
	case bowline.Unauthenticated:
	}
}
```

`Encode` takes `agent.Anthropic`, `agent.OpenAI`, or `agent.Schema` and returns JSON. No provider SDK is needed, and adding a new provider means adding one more format. `HandlerCaller(h, headers)` dispatches into an `http.Handler` in the same process. This is how the tests run against a router without opening a socket. `Result.Error` is a `*bowline.Error`, so callers can switch on the code rather than parsing text. Connection failures are returned as `UNAVAILABLE` rather than as a Go error, so an agent loop can treat them like any other tool failure.

## TypeScript

Package `@bowlinedev/agent`, built on `@bowlinedev/client`.

sketch: the same tour in TypeScript; `./bowline.contract.json` is whatever path the application generates to

```ts
import { createDispatcher, recordingTracer, toAnthropic, tools } from "@bowlinedev/agent";
import type { ContractDocument } from "@bowlinedev/client";
import contract from "./bowline.contract.json";

const exposed = tools(contract as ContractDocument, { scopes: ["billing"] });
const definitions = toAnthropic(exposed);

const dispatcher = createDispatcher(contract as ContractDocument, exposed, {
  url: "http://localhost:8080/api",
  headers: { authorization: "Bearer dev" },
  tracer: recordingTracer((line) => process.stdout.write(`${line}\n`)),
});

const result = await dispatcher.dispatch({ id: "1", tool: "invoices_get", input: { id: 3 } });
if (result.error) {
  console.log(result.error.code);
}
```

`ContractDocument` in `@bowlinedev/client` is the type of the parsed contract. The dispatcher calls procedures through the client's own transport, so the error mapping is the same one the browser client uses. `Tool.inputSchema` is `Record<string, unknown>`, not `any`.

## Python

Package `bowline-agent`, import name `bowline_agent`. Requires Python 3.11 or later and has no runtime dependencies.

```python
from bowline_agent import Call, Dispatcher, RecordingTracer, load_contract, to_openai, tools

contract = load_contract("api/bowline.contract.json")
exposed = tools(contract, scopes=["billing"])
definitions = to_openai(exposed)

dispatcher = Dispatcher(
    exposed,
    "http://localhost:8080/api",
    headers={"Authorization": "Bearer dev"},
    tracer=RecordingTracer(print),
)
result = dispatcher.dispatch(Call(id="1", tool="invoices_get", input={"id": 3}))
if result.error:
    print(result.error["code"])
```

The dispatcher uses `urllib.request` with an opener that can be injected. This is how the tests run without a network. It sends every call as `POST`, which the runtime accepts for every procedure.

## Parity

The three packages are tested against the goldens in `cmd/bowline/internal/tools/testdata`, which are generated from the ledger contract. The Anthropic and OpenAI definitions they produce are identical to each other and to the output of `bowline export tools`. Each package has a recording tracer that writes one JSON line per call, in the form `{"id","tool","input","output","error","durationMs"}`. `bowline eval record --agent` reads these lines and replays them against the mock server, recorded fixtures, or a live handler. See `evals.md`.

Sources: `agent/agent_test.go`, `packages/agent/src/tools.test.ts`, `python/bowline-agent/tests/test_encode.py`.
