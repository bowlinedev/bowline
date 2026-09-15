defmodule BowlineClient.Issue do
  @moduledoc "One validation failure: the JSON path, the rule, and the message the server would send."

  @type t :: %__MODULE__{path: [String.t()], rule: String.t(), message: String.t()}

  defstruct path: [], rule: "", message: ""

  @spec new([String.t()], String.t(), String.t()) :: t()
  def new(path, rule, message), do: %__MODULE__{path: path, rule: rule, message: message}

  @spec from_map(map()) :: t()
  def from_map(%{} = map) do
    %__MODULE__{
      path: Enum.map(Map.get(map, "path") || [], &to_string/1),
      rule: to_string(Map.get(map, "rule") || ""),
      message: to_string(Map.get(map, "message") || "")
    }
  end
end
