# Mock fixtures

`bowline mock --record` writes one file per distinct interaction under `mocks/<procedure>/<hash>.json`, where `<hash>` is the hex SHA-256 of the canonical input: the request JSON decoded and re-encoded with sorted keys and no whitespace, `{}` when the request carried no input.

```json
{
  "bowline": "1.2",
  "procedure": "invoices.get",
  "recordedAt": "2026-09-15T12:00:00Z",
  "request": { "method": "GET", "input": { "id": 3 } },
  "response": {
    "status": 200,
    "headers": { "Content-Type": "application/json; charset=utf-8" },
    "body": { "id": 3, "status": "sent", "total": "USD 1500.00" }
  }
}
```

| Field | Meaning |
|---|---|
| `bowline` | Contract format version at recording time. |
| `procedure` | Dotted procedure path. |
| `recordedAt` | UTC time of the recording. |
| `request.method` | The procedure's contract method. |
| `request.input` | The canonical input. |
| `response.status` | Upstream status code. |
| `response.headers` | `Content-Type`, `Deprecation`, and `Sunset` when the upstream sent them. |
| `response.body` | The upstream body; a non-JSON body is stored as a JSON string. |

Replay matches by procedure and canonical input and serves the recorded status, headers, and body. A fixture is never overwritten by a later recording of the same interaction; delete it to re-record. Subscriptions, uploads, and event-stream responses are proxied but not recorded.
