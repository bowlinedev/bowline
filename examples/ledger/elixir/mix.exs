defmodule LedgerElixir.MixProject do
  use Mix.Project

  def project do
    [
      app: :ledger_elixir,
      version: "1.1.0",
      elixir: "~> 1.18",
      start_permanent: Mix.env() == :prod,
      deps: deps()
    ]
  end

  def application do
    [extra_applications: [:logger]]
  end

  defp deps do
    [{:bowline_client, path: "../../../packages/elixir/bowline_client"}]
  end
end
