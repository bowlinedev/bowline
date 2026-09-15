defmodule BowlineClient.SSE do
  @moduledoc "Parser for the server-sent event framing the Bowline runtime writes."

  @type event :: {:message, String.t()} | {:error, String.t()} | :done

  @spec parse(String.t(), String.t()) :: {[event()], String.t()}
  def parse(buffer, chunk) do
    {blocks, rest} = split_blocks(buffer <> chunk)
    {Enum.flat_map(blocks, &parse_block/1), rest}
  end

  defp split_blocks(data) do
    parts = String.split(data, ["\r\n\r\n", "\n\n"])
    {complete, [rest]} = Enum.split(parts, length(parts) - 1)
    {complete, rest}
  end

  defp parse_block(block) do
    {name, payload} =
      block
      |> String.split(["\r\n", "\n"])
      |> Enum.reduce({nil, []}, fn line, {name, payload} ->
        cond do
          String.starts_with?(line, ":") -> {name, payload}
          String.starts_with?(line, "event:") -> {trim(line, "event:"), payload}
          String.starts_with?(line, "data:") -> {name, [trim(line, "data:") | payload]}
          true -> {name, payload}
        end
      end)

    data = payload |> Enum.reverse() |> Enum.join("\n")

    case name do
      "message" -> [{:message, data}]
      "error" -> [{:error, data}]
      "done" -> [:done]
      _ -> []
    end
  end

  defp trim(line, prefix) do
    line |> String.replace_prefix(prefix, "") |> String.trim_leading()
  end
end
