# bowline-agent

Turn a [Bowline](https://github.com/bowlinedev/bowline) API into tools for an LLM agent, in Python.

Bowline turns your Go code into typed API clients. This package turns the same contract into tool definitions, and calls them.

## Install

```bash
pip install bowline-agent
```

## Use

```python
from bowline_agent import to_anthropic, create_dispatcher

tools = to_anthropic(contract)
dispatch = create_dispatcher(client, contract)
```

`to_anthropic`, `to_openai` and `to_json_schema` build the tool list. `create_dispatcher` runs a tool call against your API.

Only procedures you mark as tools in Go are included.

Docs: [bowlinedev/bowline](https://github.com/bowlinedev/bowline)

Apache-2.0
