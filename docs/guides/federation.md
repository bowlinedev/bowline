# Federation

When several Go services each publish a contract, a gateway can compose them into one document. A browser then gets a single typed client with `client.ledger.invoices.get` next to `client.billing.charges.create`, and each call is proxied to the service that owns it.

## Composing

Composition is a pure function over contract documents, so none of it is specific to the gateway. Each service's procedures are moved under the service name, its type IDs get a `service:` prefix, and if two services share a type name, both are renamed with the service name in front.

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
      "version": "sha256:d42858681c4e95b20b1f05ef6427f059d1f336ad786e0c224d4b72ffa0f63842"
    },
    "billing": {
      "url": "http://127.0.0.1:8081/api",
      "contract": "../billing/api/bowline.contract.json",
      "version": "sha256:7c40b5b79482adbf6645409ce4a30de9e5e693f092fb8e3c3eed288c5a3cd77c"
    }
  }
}
```

`version` is required and holds the upstream's contract hash. The gateway refuses to start if a resolved contract does not match its pin, and `.bowline/ready` reports the mismatch per service. This means a service that redeploys with a changed API cannot silently change the client's API. Contract paths are resolved relative to the config file, not the working directory.

```bash
bowline gateway compose -o composed.contract.json
bowline gen --from composed.contract.json
```

The first command writes the composed document. The second renders whatever targets `bowline.json` lists from it. This is how the federation web app gets its client without having a Go module of its own.

## Serving

`bowline gateway` resolves, composes, and listens. Each call is routed to the service named by the first path segment, with that segment removed. `POST /api/billing.charges.create` becomes `POST /api/charges.create` on billing. Only allowlisted headers are forwarded, `X-Forwarded-*` headers are set, error envelopes pass through unchanged, subscriptions stream with a flush after each event, and uploads stream without buffering. A `GET` query is retried twice on a connection error or a 502, 503, or 504. Nothing else is ever retried, because nothing else is idempotent by definition.

The gateway also serves `.bowline/contract`, `.bowline/health`, and `.bowline/ready` for the composed document.

## Calling between services

Services still call each other directly, not through the gateway. Billing reads an invoice from the ledger through the generated Go client, with a signature:

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

The ledger's `NOT_FOUND` is turned into billing's own declared `UnknownInvoice` variant. Billing's clients then see a typed failure that belongs to billing's contract rather than an upstream error leaking through. See `signing.md` for the signature itself.

To verify this yourself: `cd examples/federation/billing && go test ./...` covers the signed call, the rejection of an unsigned one, and the variant mapping. `pnpm --filter federation-web test:e2e` starts the ledger, billing, and the gateway, then drives a browser to create an invoice and charge it through one client.
