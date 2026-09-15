# Eval recordings

`bowline eval record` runs a scripted sequence of tool calls against a running Bowline handler and writes the results to a JSON file. `bowline eval replay` re-issues the same calls and reports every result that differs. The format is versioned by the `eval` field; this document describes version 1.

## Document

```json
{
  "bowline": "1.2",
  "eval": 1,
  "contract": "sha256:4f1c…",
  "recordedAt": "2026-09-15T12:00:00Z",
  "volatile": ["createdAt", "updatedAt"],
  "steps": [
    {
      "id": "1",
      "tool": "invoices_list",
      "input": { "limit": 2 },
      "output": { "items": [{ "id": 3, "status": "sent", "total": "USD 1500.00" }], "nextCursor": "3" },
      "isError": false,
      "durationMs": 4
    },
    {
      "id": "2",
      "tool": "invoices_get",
      "input": { "id": 999 },
      "error": { "code": "NOT_FOUND", "message": "invoice 999 not found" },
      "isError": true,
      "durationMs": 2
    }
  ]
}
```

| Field | Meaning |
|---|---|
| `bowline` | Contract format version the recording was made with. |
| `eval` | Recording format version. Always `1`. |
| `contract` | The `hash` of the contract document at recording time. Replay refuses a different hash before making any call. |
| `recordedAt` | UTC time of the recording, to the second. |
| `volatile` | Object keys removed at any depth from outputs before recording and before comparing, sorted. |
| `steps` | The calls in execution order. |

Each step carries the tool name as listed by `bowline export tools`, the input as sent, and either `output` or `error`. `isError` is true exactly when `error` is present. Outputs are written after normalization: volatile keys removed, object keys sorted, numbers in canonical form. `durationMs` is informational and never compared.

An error step holds the Bowline error envelope: `code`, `message`, and, when present, `type`, `details`, and `issues`. A transport failure is recorded as `UNAVAILABLE`; a non-2xx response that is not a Bowline envelope is recorded as `UNKNOWN`.

## Scripts

`bowline eval record --script` reads a script:

```json
{
  "volatile": ["createdAt", "updatedAt"],
  "calls": [
    { "id": "1", "tool": "invoices_list", "input": { "limit": 2 } },
    { "id": "2", "tool": "invoices_get", "input": { "id": 999 } }
  ]
}
```

`id` defaults to the one-based position. `--volatile` flags add to the script's list. With `--agent`, the calls come from standard input as JSON lines instead, one object per line with at least `tool` and `input`; the `RecordingTracer` in every agent SDK writes exactly that stream, so a model-driven session can be captured and later replayed without the model.

## Replay rules

1. The recording's `contract` must equal the hash of the local contract, otherwise replay fails naming both hashes.
2. Steps run strictly in order against the live handler with their recorded inputs.
3. `isError` must match. For error steps, `error.code` must match; `error.message` is compared only with `--strict-messages`.
4. For successful steps the live output is normalized with the recording's `volatile` list and compared structurally. Every difference is reported as a JSON Pointer into the recording, for example `/steps/0/output/items/0/total`.

Replay exits 0 when nothing differs and 1 otherwise; the mismatches are printed one per line on standard error.
