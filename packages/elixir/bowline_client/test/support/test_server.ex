defmodule BowlineClient.TestServer do
  @moduledoc false

  def start(handler) do
    {:ok, listen} = :gen_tcp.listen(0, [:binary, packet: :raw, active: false, reuseaddr: true])
    {:ok, port} = :inet.port(listen)
    parent = self()
    pid = spawn_link(fn -> loop(listen, handler, parent) end)
    {"http://127.0.0.1:#{port}", pid, listen}
  end

  def stop({_url, pid, listen}) do
    Process.unlink(pid)
    Process.exit(pid, :kill)
    :gen_tcp.close(listen)
  end

  defp loop(listen, handler, parent) do
    case :gen_tcp.accept(listen) do
      {:ok, socket} ->
        request = read_request(socket)
        send(parent, {:request, request})
        respond(socket, handler.(request))
        :gen_tcp.close(socket)
        loop(listen, handler, parent)

      {:error, _} ->
        :ok
    end
  end

  defp read_request(socket, acc \\ "") do
    {:ok, data} = :gen_tcp.recv(socket, 0, 5_000)
    acc = acc <> data

    case String.split(acc, "\r\n\r\n", parts: 2) do
      [head, body] ->
        [request_line | header_lines] = String.split(head, "\r\n")
        [method, target, _] = String.split(request_line, " ", parts: 3)

        headers =
          Map.new(header_lines, fn line ->
            [name, value] = String.split(line, ":", parts: 2)
            {String.downcase(name), String.trim(value)}
          end)

        length = headers |> Map.get("content-length", "0") |> String.to_integer()
        body = read_body(socket, body, length)
        uri = URI.parse(target)

        %{
          method: method,
          path: uri.path,
          query: URI.decode_query(uri.query || ""),
          headers: headers,
          body: body
        }

      _ ->
        read_request(socket, acc)
    end
  end

  defp read_body(_socket, body, length) when byte_size(body) >= length, do: body

  defp read_body(socket, body, length) do
    {:ok, more} = :gen_tcp.recv(socket, 0, 5_000)
    read_body(socket, body <> more, length)
  end

  defp respond(socket, {:delay, ms, response}) do
    Process.sleep(ms)
    respond(socket, response)
  end

  defp respond(socket, {:stream, chunks}) do
    :gen_tcp.send(
      socket,
      "HTTP/1.1 200 OK\r\ncontent-type: text/event-stream\r\nconnection: close\r\n\r\n"
    )

    Enum.each(chunks, fn chunk ->
      :gen_tcp.send(socket, chunk)
      Process.sleep(5)
    end)
  end

  defp respond(socket, {status, body}) do
    respond(socket, {status, [{"content-type", "application/json; charset=utf-8"}], body})
  end

  defp respond(socket, {status, headers, body}) do
    header_lines = Enum.map_join(headers, "", fn {k, v} -> "#{k}: #{v}\r\n" end)

    :gen_tcp.send(
      socket,
      "HTTP/1.1 #{status} X\r\n#{header_lines}content-length: #{byte_size(body)}\r\nconnection: close\r\n\r\n#{body}"
    )
  end
end
