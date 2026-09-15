# Elixir

The `elixir` target generates one file, `bowline.ex`, holding a module per type under `<Root>.Types`, a module per router node, and a `<Root>.Client` that builds the transport. The runtime is the `bowline_client` package.

```json
{ "targets": { "elixir": { "out": "elixir/lib/bowline.ex", "package": "Ledger" } } }
```

```elixir
  defp deps do
    [{:bowline_client, "~> 0.5"}]
  end
```

The `package` option names the root module, so `Ledger.Invoices.list/3` and `Ledger.Types.Invoice` come from a contract generated with `"package": "Ledger"`.

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

Every procedure is a function on its mount's module taking the transport, an input struct, and optional call options, returning `{:ok, output}` or `{:error, %BowlineClient.Error{}}`; a bang variant raises instead. Subscriptions return a lazy stream of decoded messages, enumerated in the process that opened it, and uploads take an enumerable body and a file name.

## Errors

source: examples/ledger/elixir/lib/ledger_cli.ex:47-50

```elixir
  defp fail(%Error{} = error) do
    IO.puts(:stderr, Exception.message(error))
    exit({:shutdown, 1})
  end
```

`BowlineClient.Error` is an exception struct with `code` as one of the sixteen code atoms plus `:unknown`, the message, the HTTP status, the declared variant, its raw `details`, and `issues` carrying the same paths and messages the server produces. A network failure is `:unavailable` with status 0 and a timeout is `:deadline_exceeded`.

## Types

Struct fields are `snake_case` atoms while the JSON name stays on the wire, so `createdAt` decodes into `:created_at`. Every integer is `integer()`, with string-encoded 64-bit values parsed from and serialized to decimal strings; timestamps are `DateTime` in UTC; durations are nanoseconds; bytes are binaries decoded from base64; string enums are atoms with `from_value/1` and `to_value/1`; generic types take decoder and encoder functions for their parameters. The full table is the Elixir column of `spec/mapping-table.md`.

Proof: `cd examples/ledger/elixir && mix test` starts the Go server and drives list, create, validation issues, void, and a live `invoices.watch` subscription; `scripts/check-goldens.sh elixir` compiles a golden for every fidelity row warnings-free.
