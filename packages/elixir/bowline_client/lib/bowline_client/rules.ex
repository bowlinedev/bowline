defmodule BowlineClient.Rules do
  @moduledoc "Validation rules with the same semantics and messages as the Bowline server."

  alias BowlineClient.Issue

  @email ~r/^[^@\s]+@[^@\s]+\.[^@\s]+$/
  @uuid ~r/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/

  @spec required([String.t()], term()) :: [Issue.t()]
  def required(path, value) do
    if empty?(value), do: [Issue.new(path, "required", "is required")], else: []
  end

  @spec min([String.t()], term(), number()) :: [Issue.t()]
  def min(path, value, bound) do
    {size, unit} = size(value)

    if size < bound,
      do: [Issue.new(path, "min", "must be at least #{format(bound)}#{unit}")],
      else: []
  end

  @spec max([String.t()], term(), number()) :: [Issue.t()]
  def max(path, value, bound) do
    {size, unit} = size(value)

    if size > bound,
      do: [Issue.new(path, "max", "must be at most #{format(bound)}#{unit}")],
      else: []
  end

  @spec len([String.t()], term(), number()) :: [Issue.t()]
  def len(path, value, bound) do
    {size, unit} = size(value)

    if size != bound,
      do: [Issue.new(path, "len", "must be exactly #{format(bound)}#{unit}")],
      else: []
  end

  @spec one_of([String.t()], term(), [String.t()]) :: [Issue.t()]
  def one_of(path, value, options) do
    if to_string(value) in options,
      do: [],
      else: [Issue.new(path, "oneof", "must be one of " <> Enum.join(options, " "))]
  end

  @spec email([String.t()], String.t()) :: [Issue.t()]
  def email(path, value) do
    if email?(value), do: [], else: [Issue.new(path, "email", "must be a valid email address")]
  end

  @spec url([String.t()], String.t()) :: [Issue.t()]
  def url(path, value) do
    if url?(value), do: [], else: [Issue.new(path, "url", "must be a valid URL")]
  end

  @spec uuid([String.t()], String.t()) :: [Issue.t()]
  def uuid(path, value) do
    if uuid?(value), do: [], else: [Issue.new(path, "uuid", "must be a valid UUID")]
  end

  @spec email?(term()) :: boolean()
  def email?(value) when is_binary(value), do: Regex.match?(@email, value)
  def email?(_), do: false

  @spec url?(term()) :: boolean()
  def url?(value) when is_binary(value) do
    case URI.new(value) do
      {:ok, %URI{scheme: scheme, host: host}} when is_binary(scheme) and is_binary(host) ->
        scheme != "" and host != ""

      _ ->
        false
    end
  end

  def url?(_), do: false

  @spec uuid?(term()) :: boolean()
  def uuid?(value) when is_binary(value), do: Regex.match?(@uuid, value)
  def uuid?(_), do: false

  defp empty?(nil), do: true
  defp empty?(""), do: true
  defp empty?(0), do: true
  defp empty?(false), do: true
  defp empty?([]), do: true
  defp empty?(value) when is_float(value), do: value == 0.0
  defp empty?(map) when is_map(map) and not is_struct(map), do: map_size(map) == 0
  defp empty?(_), do: false

  defp size(value) when is_binary(value), do: {String.length(value), " characters"}
  defp size(value) when is_list(value), do: {length(value), " items"}
  defp size(value) when is_map(value) and not is_struct(value), do: {map_size(value), " items"}
  defp size(value) when is_number(value), do: {value, ""}
  defp size(_), do: {0, ""}

  defp format(bound) when is_float(bound) and bound == trunc(bound),
    do: Integer.to_string(trunc(bound))

  defp format(bound), do: to_string(bound)
end
