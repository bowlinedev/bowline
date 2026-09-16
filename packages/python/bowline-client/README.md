# bowline-client

The Python client runtime for [Bowline](https://github.com/bowlinedev/bowline).

Bowline turns your Go code into typed API clients. This package is what the generated `bowline.py` uses to make calls.

## Install

```bash
pip install bowline-client
```

## Use

```python
from bowline_client import Transport
from bowline import Client

client = Client(Transport("http://localhost:8080/api"))
invoice = client.invoices.get(GetInput(id=3))
```

You do not write the types. `bowline gen` writes them from your Go code.

Errors come back as `BowlineError` with the same codes the Go server uses. Queries are sent as `GET`, mutations as `POST`. Inputs are checked before the request leaves.

Docs: [bowlinedev/bowline](https://github.com/bowlinedev/bowline)

Apache-2.0
