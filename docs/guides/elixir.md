# Elixir

The `elixir` target generates a single file, `bowline.ex`, containing a module per type under `<Root>.Types`, a module per router node, and a `<Root>.Client` module that builds the transport. The runtime is the `bowline_client` package.

```json
{ "targets": { "elixir": { "out": "elixir/lib/bowline.ex", "package": "Ledger" } } }
```

```elixir
  defp deps do
    [{:bowline_client, "~> 1.0"}]
  end
```

The `package` option sets the root module name. With `"package": "Ledger"`, you get `Ledger.Invoices.list/3` and `Ledger.Types.Invoice`.

## Calling

source: examples/ledger/elixir/lib/ledger_cli.ex:9-19

```elixir
  def transport do
    Ledger.Client.new(System.get_env("LEDGER_URL", "http://localhost:8080/api"))
  end

  @spec list(BowlineClient.Transport.t()) :: :ok
  def list(transport) do
    case Invoices.list(transport, %Types.ListInvoicesInput{limit: 20}) do
      {:ok, page} -> Enum.each(page.items, &print/1)
      {:error, error} -> fail(error)
    end
  end
```

Every procedure is a function on its mount's module. It takes the transport, an input struct, and optional call options, and returns `{:ok, output}` or `{:error, %BowlineClient.Error{}}`. There is also a bang variant that raises. Subscriptions return a lazy stream of decoded messages, enumerated in the process that opened it. Uploads take an enumerable body and a file name.

## Errors

source: examples/ledger/elixir/lib/ledger_cli.ex:47-50

```elixir
  defp fail(%Error{} = error) do
    IO.puts(:stderr, Exception.message(error))
    exit({:shutdown, 1})
  end
```

`BowlineClient.Error` is an exception struct. `code` is one of the sixteen code atoms, plus `:unknown`. It also has the message, the HTTP status, the declared variant, its raw `details`, and `issues` with the same paths and messages the server produces. A network failure is `:unavailable` with status 0. A timeout is `:deadline_exceeded`.

## Types

Struct fields are `snake_case` atoms, while the JSON name stays on the wire, so `createdAt` decodes into `:created_at`. Every integer is `integer()`. String-encoded 64-bit values are parsed from and serialized to decimal strings. Timestamps are `DateTime` in UTC. Durations are nanoseconds. Bytes are binaries decoded from base64. String enums are atoms with `from_value/1` and `to_value/1`. Generic types take decoder and encoder functions for their type parameters. The full table is the Elixir column of `spec/mapping-table.md`.

To verify: `cd examples/ledger/elixir && mix test` starts the Go server and runs list, create, validation issues, void, and a live `invoices.watch` subscription. `scripts/check-goldens.sh elixir` compiles a golden for every fidelity row with no warnings.
