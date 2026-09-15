defmodule LedgerTest do
  use ExUnit.Case

  alias BowlineClient.Error
  alias Ledger.Invoices
  alias Ledger.Types

  setup_all do
    port = free_port()
    ledger_dir = Path.expand("../..", __DIR__)

    server =
      Port.open({:spawn_executable, System.find_executable("go")}, [
        :binary,
        :exit_status,
        args: ["run", "./cmd/server"],
        cd: ledger_dir,
        env: [
          {~c"ADDR", ~c"127.0.0.1:#{port}"},
          {~c"LEDGER_FIXED_TIME", ~c"2026-09-15T12:00:00Z"}
        ]
      ])

    {:os_pid, os_pid} = Port.info(server, :os_pid)
    base_url = "http://127.0.0.1:#{port}/api"
    wait_for_health(base_url, 300)

    on_exit(fn ->
      System.cmd("pkill", ["-P", Integer.to_string(os_pid)], stderr_to_stdout: true)
      System.cmd("kill", [Integer.to_string(os_pid)], stderr_to_stdout: true)
    end)

    {:ok, transport: Ledger.Client.new(base_url)}
  end

  test "lists the two seeded invoices", %{transport: transport} do
    assert {:ok, %Types.Page{items: items}} =
             Invoices.list(transport, %Types.ListInvoicesInput{limit: 20})

    seeded = Enum.filter(items, &(&1.id in [3, 4]))
    assert Enum.map(seeded, & &1.id) == [3, 4]
    assert [%Types.Invoice{status: :sent, total: "USD 1500.00"} | _] = seeded
    assert %DateTime{} = hd(seeded).created_at
  end

  test "a bad create surfaces the server's validation issues", %{transport: transport} do
    input = %Types.CreateInvoiceInput{
      customer_id: 1,
      lines: [%Types.Line{description: "", quantity: 0, unit_price: "USD 10.00"}]
    }

    assert {:error, %Error{code: :invalid_argument, status: 400, issues: issues}} =
             Invoices.create(transport, input)

    assert Enum.map(issues, & &1.path) == [
             ["lines", "0", "description"],
             ["lines", "0", "quantity"]
           ]

    assert Types.CreateInvoiceInput.validate(input) |> Enum.map(& &1.path) ==
             Enum.map(issues, & &1.path)
  end

  test "creates, watches, and voids an invoice", %{transport: transport} do
    task =
      Task.async(fn ->
        {:ok, stream} = Invoices.watch(transport, %Types.WatchInput{})
        stream |> Enum.take(1) |> hd()
      end)

    Process.sleep(200)

    input = %Types.CreateInvoiceInput{
      customer_id: 1,
      lines: [%Types.Line{description: "Widgets", quantity: 3, unit_price: "USD 10.00"}],
      note: "net 30"
    }

    assert {:ok, %Types.Invoice{} = created} = Invoices.create(transport, input)
    assert created.total == "USD 30.00"
    assert created.status == :draft
    assert created.note == "net 30"

    assert %Types.Invoice{id: watched_id} = Task.await(task, 5_000)
    assert watched_id == created.id

    assert {:ok, %Types.Invoice{status: :void}} =
             Invoices.void(transport, %Types.VoidInvoiceInput{id: created.id})

    assert {:error, %Error{code: :failed_precondition, type: "InvoiceLocked"} = error} =
             Invoices.void(transport, %Types.VoidInvoiceInput{id: 4})

    assert %Types.InvoiceLocked{id: 4, status: :paid} = Ledger.Client.error_details(error)
  end

  defp free_port do
    {:ok, socket} = :gen_tcp.listen(0, [])
    {:ok, port} = :inet.port(socket)
    :gen_tcp.close(socket)
    port
  end

  defp wait_for_health(_base_url, 0), do: raise("the ledger server did not start")

  defp wait_for_health(base_url, attempts) do
    case Req.get(base_url <> "/health", retry: false) do
      {:ok, %Req.Response{status: 200}} ->
        :ok

      _ ->
        Process.sleep(200)
        wait_for_health(base_url, attempts - 1)
    end
  end
end
