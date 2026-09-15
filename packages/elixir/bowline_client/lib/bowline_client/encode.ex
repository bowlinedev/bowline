defmodule BowlineClient.Encode do
  @moduledoc "Encoding helpers used by generated `to_map/1` functions."

  @spec optional(map(), String.t(), term(), (term() -> term())) :: map()
  def optional(map, _key, nil, _encode), do: map
  def optional(map, key, value, encode), do: Map.put(map, key, encode.(value))

  @spec nullable(term(), (term() -> term())) :: term()
  def nullable(nil, _encode), do: nil
  def nullable(value, encode), do: encode.(value)

  @spec timestamp(DateTime.t()) :: String.t()
  def timestamp(%DateTime{} = value) do
    value |> DateTime.shift_zone!("Etc/UTC") |> DateTime.to_iso8601()
  end

  @spec bytes(binary()) :: String.t()
  def bytes(value), do: Base.encode64(value)

  @spec big_int(integer()) :: String.t()
  def big_int(value), do: Integer.to_string(value)

  @spec list([term()], (term() -> term())) :: [term()]
  def list(values, encode), do: Enum.map(values, encode)

  @spec map(map(), (term() -> term())) :: map()
  def map(values, encode), do: Map.new(values, fn {k, v} -> {to_string(k), encode.(v)} end)
end
