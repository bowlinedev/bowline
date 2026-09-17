# Recording and replaying agent runs

An agent is only as testable as the API underneath it. `bowline eval` records a sequence of tool calls and their results, and replays it later against the live API, failing on any result that changed. The model is not part of the loop, so if the app is deterministic then the replay is too.

## Recording

Write a script of calls:

```json
{
  "volatile": ["createdAt", "updatedAt"],
  "calls": [
    { "id": "1", "tool": "invoices_list", "input": { "limit": 2 } },
    { "id": "2", "tool": "invoices_get", "input": { "id": 999 } }
  ]
}
```

Then, with the server running:

```bash
bowline eval record --backend url --url http://localhost:8080/api --script evals/script.json --out evals/list-and-get.json --header "Authorization: Bearer dev"
```

The recording contains the contract hash, the time, the volatile keys, and one step per call with its normalized output or its error envelope. `spec/eval.md` documents the format. Keys named in `volatile` are removed at any depth before recording and before comparing, so that timestamps and generated identifiers do not make every replay fail.

To record a session driven by a real model, run it through an agent SDK with its recording tracer enabled, and pipe the JSON lines in:

```bash
go run ./cmd/agent 2>/dev/null | bowline eval record --backend url --url http://localhost:8080/api --agent --out evals/session.json
```

## Replaying

```bash runnable
bowline eval replay evals/list-and-get.json --backend url --url http://127.0.0.1:18080/api --header "Authorization: Bearer dev"
```

Replay first compares the contract hash in the recording with the local contract, and refuses to run if they differ. Then it re-issues each call in order and compares the results. `isError` must match, an error's code must match, and a success's output must match after normalization. Every difference is reported as a JSON Pointer into the recording:

```
mismatch  step 1: /steps/0/output/items/0/total: recorded "USD 1600.00", got "USD 1500.00"
bowline: 1 mismatch(es) replaying evals/list-and-get.json
```

Error messages are only compared with `--strict-messages`, since messages are prose and change more often than behavior does.

## Without a provider

The default backend is the contract-derived mock server, started in the same process. This means a recording can be made and replayed without any Go server running:

```bash runnable
bowline eval record --script evals/script.json --out /tmp/mock-run.json && bowline eval replay /tmp/mock-run.json
```

Generated data is deterministic for a given seed (`--seed`, default 1), so two runs against the mock produce the same steps. `--backend replay --fixtures mocks` serves fixtures recorded with `bowline mock --record` instead, and fails on any call that has no fixture. This is how an agent run can be pinned to real responses without a live upstream. `--backend url --url http://…` targets a running handler.

## In CI

The ledger keeps a recording in `examples/ledger/evals/list-and-get.json`, made with `LEDGER_FIXED_TIME=2026-09-15T12:00:00Z` and `LEDGER_TOKEN=dev`. `scripts/eval-ledger.sh` builds the CLI and the server, starts the server with that environment, and replays the recording. The `eval` workflow runs it twice on every push. After an intentional API change, run `scripts/eval-ledger.sh record` to write a new recording and commit it with the change.

The tests in `cmd/bowline/internal/eval/eval_test.go` cover the recording shape, identical and changed replays, volatile keys, strict messages, and the contract hash check.
