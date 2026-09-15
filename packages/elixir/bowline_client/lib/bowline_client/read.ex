defmodule BowlineClient.Read do
  @moduledoc "Decoding helpers used by generated `from_map/1` functions; each raises `ArgumentError` naming the key on a shape mismatch."

  alias BowlineClient.Issue

  @spec int(map(), String.t()) :: integer()
  def int(map, key), do: int_value(Map.get(map, key), key)

  @spec int_value(term(), String.t()) :: integer()
  def int_value(value, key) do
    case value do
      v when is_integer(v) -> v
      v when is_float(v) and v == trunc(v) -> trunc(v)
      nil -> 0
      other -> mismatch(key, "integer", other)
    end
  end

  @spec big_int(map(), String.t()) :: integer()
  def big_int(map, key), do: big_int_value(Map.get(map, key), key)

  @spec big_int_value(term(), String.t()) :: integer()
  def big_int_value(value, key) do
    case value do
      v when is_integer(v) ->
        v

      v when is_binary(v) ->
        case Integer.parse(v) do
          {n, ""} -> n
          _ -> mismatch(key, "decimal string", v)
        end

      nil ->
        0

      other ->
        mismatch(key, "decimal string", other)
    end
  end

  @spec float(map(), String.t()) :: float()
  def float(map, key), do: float_value(Map.get(map, key), key)

  @spec float_value(term(), String.t()) :: float()
  def float_value(value, key) do
    case value do
      v when is_float(v) -> v
      v when is_integer(v) -> v * 1.0
      nil -> 0.0
      other -> mismatch(key, "number", other)
    end
  end

  @spec bool(map(), String.t()) :: boolean()
  def bool(map, key), do: bool_value(Map.get(map, key), key)

  @spec bool_value(term(), String.t()) :: boolean()
  def bool_value(value, key) do
    case value do
      v when is_boolean(v) -> v
      nil -> false
      other -> mismatch(key, "boolean", other)
    end
  end

  @spec string(map(), String.t()) :: String.t()
  def string(map, key), do: string_value(Map.get(map, key), key)

  @spec string_value(term(), String.t()) :: String.t()
  def string_value(value, key) do
    case value do
      v when is_binary(v) -> v
      nil -> ""
      other -> mismatch(key, "string", other)
    end
  end

  @spec optional(map(), String.t(), (term() -> value)) :: value | nil when value: term()
  def optional(map, key, decode), do: optional_value(Map.get(map, key), decode)

  @spec optional_value(term(), (term() -> value)) :: value | nil when value: term()
  def optional_value(nil, _decode), do: nil
  def optional_value(value, decode), do: decode.(value)

  @spec timestamp(map(), String.t()) :: DateTime.t()
  def timestamp(map, key), do: timestamp_value(Map.get(map, key), key)

  @spec timestamp_value(term(), String.t()) :: DateTime.t()
  def timestamp_value(value, key) do
    case value do
      v when is_binary(v) -> parse_timestamp(v, key)
      nil -> ~U[0001-01-01 00:00:00Z]
      other -> mismatch(key, "RFC 3339 timestamp", other)
    end
  end

  @spec parse_timestamp(String.t(), String.t()) :: DateTime.t()
  def parse_timestamp(value, key \\ "timestamp") do
    case DateTime.from_iso8601(value) do
      {:ok, dt, _offset} -> DateTime.shift_zone!(dt, "Etc/UTC")
      _ -> mismatch(key, "RFC 3339 timestamp", value)
    end
  end

  @spec bytes(map(), String.t()) :: binary()
  def bytes(map, key), do: bytes_value(Map.get(map, key), key)

  @spec bytes_value(term(), String.t()) :: binary()
  def bytes_value(value, key) do
    case value do
      v when is_binary(v) ->
        case Base.decode64(v) do
          {:ok, decoded} -> decoded
          :error -> mismatch(key, "base64 string", v)
        end

      nil ->
        <<>>

      other ->
        mismatch(key, "base64 string", other)
    end
  end

  @spec list(map(), String.t(), (term() -> item)) :: [item] when item: term()
  def list(map, key, decode), do: list_value(Map.get(map, key), key, decode)

  @spec list_value(term(), String.t(), (term() -> item)) :: [item] when item: term()
  def list_value(value, key, decode) do
    case value do
      v when is_list(v) -> Enum.map(v, decode)
      nil -> []
      other -> mismatch(key, "list", other)
    end
  end

  @spec map(map(), String.t(), (term() -> value)) :: %{String.t() => value} when value: term()
  def map(map, key, decode), do: map_value(Map.get(map, key), key, decode)

  @spec map_value(term(), String.t(), (term() -> value)) :: %{String.t() => value}
        when value: term()
  def map_value(value, key, decode) do
    case value do
      v when is_map(v) -> Map.new(v, fn {k, item} -> {to_string(k), decode.(item)} end)
      nil -> %{}
      other -> mismatch(key, "object", other)
    end
  end

  @spec raw(map(), String.t()) :: term()
  def raw(map, key), do: Map.get(map, key)

  @spec object(term(), String.t()) :: map()
  def object(value, _key) when is_map(value),
    do: Map.new(value, fn {k, v} -> {to_string(k), v} end)

  def object(other, key), do: mismatch(key, "object", other)

  @spec validate_optional(term(), (term() -> [Issue.t()])) :: [Issue.t()]
  def validate_optional(nil, _validate), do: []
  def validate_optional(value, validate), do: validate.(value)

  @spec validate_list([String.t()], [term()] | nil, (term() -> [Issue.t()])) :: [Issue.t()]
  def validate_list(_path, nil, _validate), do: []

  def validate_list(path, items, validate) do
    items
    |> Enum.with_index()
    |> Enum.flat_map(fn {item, i} ->
      prefixed(path ++ [Integer.to_string(i)], validate.(item))
    end)
  end

  @spec validate_map([String.t()], map() | nil, (term() -> [Issue.t()])) :: [Issue.t()]
  def validate_map(_path, nil, _validate), do: []

  def validate_map(path, entries, validate) do
    Enum.flat_map(entries, fn {k, item} -> prefixed(path ++ [to_string(k)], validate.(item)) end)
  end

  @spec prefixed([String.t()], [Issue.t()]) :: [Issue.t()]
  def prefixed(prefix, issues) do
    Enum.map(issues, fn %Issue{} = issue -> %Issue{issue | path: prefix ++ issue.path} end)
  end

  @spec mismatch(String.t(), String.t(), term()) :: no_return()
  def mismatch(key, expected, value) do
    raise ArgumentError, "#{key}: expected #{expected}, got #{inspect(value)}"
  end
end
