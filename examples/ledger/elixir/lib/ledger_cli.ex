defmodule LedgerCli do
  @moduledoc "Lists, creates, and voids ledger invoices through the generated Elixir client."

  alias BowlineClient.Error
  alias Ledger.Invoices
  alias Ledger.Types

  @spec transport() :: BowlineClient.Transport.t()
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

  @spec create(BowlineClient.Transport.t(), String.t(), integer()) :: :ok
  def create(transport, description, quantity) do
    input = %Types.CreateInvoiceInput{
      customer_id: 1,
      lines: [%Types.Line{description: description, quantity: quantity, unit_price: "USD 10.00"}]
    }

    case Invoices.create(transport, input) do
      {:ok, invoice} -> print(invoice)
      {:error, error} -> fail(error)
    end
  end

  @spec void(BowlineClient.Transport.t(), integer()) :: :ok
  def void(transport, id) do
    case Invoices.void(transport, %Types.VoidInvoiceInput{id: id}) do
      {:ok, invoice} -> print(invoice)
      {:error, error} -> fail(error)
    end
  end

  defp print(%Types.Invoice{} = invoice) do
    created = Calendar.strftime(local_time(invoice.created_at), "%Y-%m-%d %H:%M")
    IO.puts("#{invoice.id}\t#{invoice.status}\t#{invoice.total}\t#{created}")
  end

  defp fail(%Error{} = error) do
    IO.puts(:stderr, Exception.message(error))
    exit({:shutdown, 1})
  end

  defp local_time(%DateTime{} = utc) do
    utc
    |> DateTime.to_naive()
    |> NaiveDateTime.to_erl()
    |> :calendar.universal_time_to_local_time()
    |> NaiveDateTime.from_erl!()
  end
end

defmodule Mix.Tasks.Ledger.List do
  @moduledoc "Lists invoices: mix ledger.list"
  @shortdoc "List invoices"
  use Mix.Task

  @impl true
  def run(_args) do
    Mix.Task.run("app.start")
    LedgerCli.list(LedgerCli.transport())
  end
end

defmodule Mix.Tasks.Ledger.Create do
  @moduledoc "Creates an invoice: mix ledger.create <description> <quantity>"
  @shortdoc "Create an invoice"
  use Mix.Task

  @impl true
  def run([description, quantity]) do
    Mix.Task.run("app.start")
    LedgerCli.create(LedgerCli.transport(), description, String.to_integer(quantity))
  end

  def run(_), do: Mix.raise("usage: mix ledger.create <description> <quantity>")
end

defmodule Mix.Tasks.Ledger.Void do
  @moduledoc "Voids an invoice: mix ledger.void <id>"
  @shortdoc "Void an invoice"
  use Mix.Task

  @impl true
  def run([id]) do
    Mix.Task.run("app.start")
    LedgerCli.void(LedgerCli.transport(), String.to_integer(id))
  end

  def run(_), do: Mix.raise("usage: mix ledger.void <id>")
end
