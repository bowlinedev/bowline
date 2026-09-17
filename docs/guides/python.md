# Python

The `python` target generates a single module, `bowline.py`, containing Pydantic models, an async client built on `httpx.AsyncClient`, and a `SyncClient` with the same methods. The runtime is `bowline-client` on PyPI.

```json
{ "targets": { "python": { "out": "python/bowline.py" } } }
```

```toml
dependencies = ["bowline-client>=1.0"]
```

## Calling

source: examples/ledger/python/ledger_cli.py:21-25

```python
def connect() -> SyncClient:
    url = os.environ.get("LEDGER_URL", "http://localhost:8080/api")
    token = os.environ.get("LEDGER_TOKEN")
    headers = (lambda: {"authorization": f"Bearer {token}"}) if token else None
    return SyncClient(SyncTransport(url, headers=headers))
```

source: examples/ledger/python/ledger_cli.py:47-53

```python
        if args.command == "list":
            page = client.invoices.list(ListInvoicesInput(limit=50))
            for invoice in page.items:
                show(invoice)
        elif args.command == "create":
            line = Line(description=args.description, quantity=args.quantity, unitPrice="USD 10.00")
            show(client.invoices.create(CreateInvoiceInput(customerId=1, lines=[line])))
```

The async `Client` has the same methods as coroutines. Subscriptions are async iterators (plain iterators on the sync client). Uploads take a `BinaryIO` and a file name. Pydantic validates inputs before the request using the contract's rules, so a bad input raises a `ValidationError` locally.

## Errors

source: examples/ledger/python/ledger_cli.py:58-69

```python
    except BowlineError as err:
        if err.type == "InvoiceLocked":
            locked = err.details_as(InvoiceLocked)
            print(
                f"invoice {locked.id} is locked because it is {locked.status.value}",
                file=sys.stderr,
            )
        else:
            print(f"{err.code.value}: {err.message}", file=sys.stderr)
            for issue in err.issues:
                print(f"  {'.'.join(issue.path)}: {issue.message}", file=sys.stderr)
        return 1
```

`BowlineError` has `code`, `message`, `status`, the declared variant in `type`, raw `details` with a typed `details_as` helper, and `issues`. Network failures are `Code.UNAVAILABLE` with status 0. Timeouts are `Code.DEADLINE_EXCEEDED`.

## Types

Field names are the JSON names, so `createdAt` stays `createdAt`. `,string` integers are `BigInt`. Timestamps are timezone-aware `datetime` values. Bytes are `Base64Bytes`. String enums subclass `str, Enum`. Generic types are `Generic[T]` models. The full table is the Python column of `spec/mapping-table.md`.

To verify: `cd examples/ledger/python && uv run pytest` starts the Go server and runs list, create, validation, void, and a subscription. `scripts/check-goldens.sh python` type-checks and round-trips a golden for every fidelity row.
