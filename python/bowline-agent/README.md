# bowline-agent

Tool definitions and dispatch for [Bowline](https://github.com/bowlinedev/bowline) APIs, with no runtime dependencies.

```python
from bowline_agent import Call, Dispatcher, load_contract, to_anthropic, tools

contract = load_contract("api/bowline.contract.json")
exposed = tools(contract, scopes=["billing"])
definitions = to_anthropic(exposed)

dispatcher = Dispatcher(
    exposed, "http://localhost:8080/api", headers={"Authorization": "Bearer dev"}
)
result = dispatcher.dispatch(Call(id="1", tool="invoices_get", input={"id": 3}))
if result.error:
    print(result.error["code"])
else:
    print(result.output)
```

`tools` reads the procedures exposed with `bowline.Tool(...)` from a contract generated with `"schemas": true`. `to_anthropic`, `to_openai`, and `to_json_schema` produce the same shapes as `bowline export tools`. `Dispatcher` forwards a `Call` to the running API and returns the output or the Bowline error envelope; a `RecordingTracer` writes one JSON line per call for `bowline eval record --agent`.
