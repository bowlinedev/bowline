defmodule BowlineClient.MixProject do
  use Mix.Project

  def project do
    [
      app: :bowline_client,
      version: "0.6.0",
      elixir: "~> 1.18",
      start_permanent: Mix.env() == :prod,
      deps: deps(),
      description: "Runtime for Bowline generated Elixir clients",
      package: package(),
      elixirc_paths: elixirc_paths(Mix.env())
    ]
  end

  def application do
    [extra_applications: [:logger]]
  end

  defp deps do
    [
      {:req, "~> 0.7"}
    ]
  end

  defp package do
    [
      licenses: ["Apache-2.0"],
      links: %{"GitHub" => "https://github.com/bowlinedev/bowline"},
      files: ~w(lib mix.exs README.md CHANGELOG.md)
    ]
  end

  defp elixirc_paths(:test), do: ["lib", "test/support"]
  defp elixirc_paths(_), do: ["lib"]
end
