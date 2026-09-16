# bowline_client

The Elixir client runtime for [Bowline](https://github.com/bowlinedev/bowline).

Bowline turns your Go code into typed API clients. This package is what the generated `bowline.ex` uses to make calls.

## Install

```elixir
def deps do
  [{:bowline_client, "~> 1.0"}]
end
```

## Use

```elixir
transport = Bowline.Transport.new("http://localhost:8080/api")
{:ok, invoice} = Bowline.Invoices.get(transport, %{id: 3})
```

You do not write the types. `bowline gen` writes them from your Go code.

Errors come back as `Bowline.Error` with the same codes the Go server uses. Queries are sent as `GET`, mutations as `POST`. Inputs are checked before the request leaves.

Docs: [bowlinedev/bowline](https://github.com/bowlinedev/bowline)

Apache-2.0
