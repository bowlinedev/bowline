# @bowlinedev/agent

Turn a [Bowline](https://github.com/bowlinedev/bowline) API into tools for an LLM agent.

Bowline turns your Go code into typed API clients. This package turns the same contract into tool definitions, and calls them.

## Install

```bash
npm install @bowlinedev/agent @bowlinedev/client
```

## Use

```ts
import { toAnthropic, createDispatcher } from "@bowlinedev/agent";

const tools = toAnthropic(contract);
const dispatch = createDispatcher(client, contract);
```

`toAnthropic`, `toOpenAI` and `toJSONSchema` build the tool list. `createDispatcher` runs a tool call against your API.

Only procedures you mark as tools in Go are included.

Docs: [docs/guides/agents.md](https://github.com/bowlinedev/bowline/blob/main/docs/guides/agents.md)

Apache-2.0
