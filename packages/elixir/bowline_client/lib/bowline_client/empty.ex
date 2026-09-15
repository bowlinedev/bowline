defmodule BowlineClient.Empty do
  @moduledoc "The input or output of a procedure declared with `struct{}`."

  @type t :: %__MODULE__{}

  defstruct []

  @spec from_map(term()) :: t()
  def from_map(_), do: %__MODULE__{}

  @spec to_map(t()) :: map()
  def to_map(%__MODULE__{}), do: %{}

  @spec validate(t()) :: [BowlineClient.Issue.t()]
  def validate(%__MODULE__{}), do: []
end
