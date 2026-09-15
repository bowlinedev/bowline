**4 contract change(s), 3 breaking**

| Category | Path | Change | Consumers |
|---|---|---|---|
| breaking | `procedure customers.get` | method changed from GET to POST | unused by consumers |
| breaking | `procedure invoices.attach output field id` | field removed | unused by consumers |
| breaking | `procedure invoices.create output field total` | field removed | breaks `ledger-web` (1 interactions) |
| narrowed | `procedure invoices.void error InvoiceLocked` | error variant removed | — |

breaking  procedure customers.get: method changed from GET to POST; unused by consumers
breaking  procedure invoices.attach output field id: field removed; unused by consumers
breaking  procedure invoices.create output field total: field removed; breaks ledger-web (1 interactions)
narrowed  procedure invoices.void error InvoiceLocked: error variant removed
