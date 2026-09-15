# Federation

Several Go services each publish a contract. A gateway composes them into one document, so a browser gets a single typed client with `client.ledger.invoices.get` beside `client.billing.charges.create`, and every call is proxied to the service that owns it.

## Composing

Composition is a pure function over contract documents, so nothing about it is specific to the gateway: each service's procedures move under its name, its type IDs gain a `service:` prefix, and a type name that two services share is renamed with the service in front.

```
ledger.invoices.get      billing.charges.create
Invoice                  Charge
Ledger_Status            Billing_Status
```

The gateway config names each upstream, the file or registry its contract comes from, and the hash it is pinned to.

source: examples/federation/gateway/bowline.gateway.json:1-17

```json
{
  "listen": "127.0.0.1:8090",
  "prefix": "/api",
  "timeout": "30s",
  "services": {
    "ledger": {
      "url": "http://127.0.0.1:8080/api",
      "contract": "../../ledger/api/bowline.contract.json",
      "version": "sha256:6ae8f5af6bb2a89fc3096c55e7ccc9b2443b14edc0aaca899c21fdf767d8c816"
    },
    "billing": {
      "url": "http://127.0.0.1:8081/api",
      "contract": "../billing/api/bowline.contract.json",
      "version": "sha256:3cc015a9c234932fadd909019c94570725a37177c8b80eeaa5767a0d2f7b637f"
    }
  }
}
```

`version` is required and is the upstream's contract hash. The gateway refuses to start when a resolved contract does not match its pin, and `.bowline/ready` reports the mismatch per service, so a service that redeploys with a changed API cannot silently change the client's API. Contract paths are resolved against the config file, not the working directory.

```bash
bowline gateway compose -o composed.contract.json
bowline gen --from composed.contract.json
```

The first command writes the composed document; the second renders whatever targets `bowline.json` declares from it, which is how the federation web app gets its client without a Go module of its own.

## Serving

`bowline gateway` resolves, composes, and listens. Each call goes to the service named by the first path segment, with the segment stripped: `POST /api/billing.charges.create` becomes `POST /api/charges.create` on billing. Only allowlisted headers cross, `X-Forwarded-*` are set, error envelopes pass through untouched, subscriptions stream with a flush per event, and uploads stream without buffering. A `GET` query retries twice on a connection error or a 502, 503, or 504; nothing else ever retries, because nothing else is idempotent by definition.

The gateway also serves `.bowline/contract`, `.bowline/health`, and `.bowline/ready` for the composed document.

## Calling between services

Services still call each other directly, not through the gateway. Billing reads an invoice from the ledger through the generated Go client, signed:

source: examples/federation/billing/api/ledger.go:10-17

```go
func LedgerClient(url, keyID string, secret []byte) *ledgerclient.Client {
	if keyID == "" || len(secret) == 0 {
		return ledgerclient.New(url)
	}
	return ledgerclient.New(url, ledgerclient.WithHTTPClient(&http.Client{
		Transport: &signing.Transport{KeyID: keyID, Secret: secret},
	}))
}
```

source: examples/federation/billing/api/routes.go:86-95

```go
	invoice, err := a.ledger.Invoices.Get(ctx, ledgerclient.GetInvoiceInput{ID: in.InvoiceID})
	if err != nil {
		var remote *bowline.Error
		if errors.As(err, &remote) && remote.Code == bowline.NotFound {
			return billing.Charge{}, UnknownInvoice(in)
		}
		return billing.Charge{}, err
	}
	return a.store.Create(invoice.ID, invoice.Total), nil
}
```

The ledger's `NOT_FOUND` becomes billing's declared `UnknownInvoice` variant, so billing's own clients see a typed failure that belongs to billing's contract rather than a leaked upstream error. See `signing.md` for the signature itself.

Proof: `cd examples/federation/billing && go test ./...` covers the signed call, the rejection of an unsigned one, and the variant mapping; `pnpm --filter federation-web test:e2e` starts the ledger, billing, and the gateway, then drives the browser to create an invoice and charge it through one client.
