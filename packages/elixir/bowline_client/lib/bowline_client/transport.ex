defmodule BowlineClient.Transport do
  @moduledoc """
  Sends procedure calls to a Bowline handler over HTTP and maps every outcome
  to `{:ok, value}` or `{:error, %BowlineClient.Error{}}`.

  Queries go out as `GET` with the input in the `input` query parameter;
  mutations and sensitive queries go out as `POST` with a JSON body.
  """

  alias BowlineClient.Error
  alias BowlineClient.SSE

  @type method :: :get | :post
  @type headers ::
          %{optional(String.t()) => String.t()} | (-> %{optional(String.t()) => String.t()})
  @type call_opts :: [headers: %{optional(String.t()) => String.t()}, timeout: pos_integer()]
  @type decoder :: (term() -> term())

  @type t :: %__MODULE__{
          base_url: String.t(),
          headers: headers(),
          req: Req.Request.t()
        }

  defstruct base_url: "", headers: %{}, req: nil

  @doc """
  Builds a transport for the API served at `base_url`.

  Options: `:headers` as a map or a zero-arity function evaluated per call, and
  `:req` as extra options for `Req.new/1`.
  """
  @spec new(keyword()) :: t()
  def new(opts) do
    base_url = opts |> Keyword.fetch!(:base_url) |> String.trim_trailing("/")
    req_opts = Keyword.get(opts, :req, [])

    %__MODULE__{
      base_url: base_url,
      headers: Keyword.get(opts, :headers, %{}),
      req: Req.new(Keyword.merge([retry: false], req_opts))
    }
  end

  @doc "Calls a query or mutation and decodes the output with `decode`."
  @spec call(t(), String.t(), method(), map(), decoder(), call_opts()) ::
          {:ok, term()} | {:error, Error.t()}
  def call(%__MODULE__{} = transport, path, method, input, decode, opts \\ []) do
    request = build(transport, path, method, input, "application/json", opts)

    case run(request, path) do
      {:ok, %Req.Response{status: status, body: body}} when status in 200..299 ->
        decode_ok(body, decode)

      {:ok, %Req.Response{status: status, body: body}} ->
        {:error, envelope(status, body)}

      {:error, error} ->
        {:error, error}
    end
  end

  @doc "Opens a subscription and returns a lazy stream of decoded messages."
  @spec subscribe(t(), String.t(), map(), decoder(), call_opts()) ::
          {:ok, Enumerable.t()} | {:error, Error.t()}
  def subscribe(%__MODULE__{} = transport, path, input, decode, opts \\ []) do
    request = build(transport, path, :get, input, "text/event-stream", opts)

    case run(Req.merge(request, into: :self), path) do
      {:ok, %Req.Response{status: status} = response} when status in 200..299 ->
        {:ok, stream(response, decode, path)}

      {:ok, %Req.Response{status: status} = response} ->
        {:error, envelope(status, collect(response))}

      {:error, error} ->
        {:error, error}
    end
  end

  @doc "Sends an upload: the JSON input as the first multipart part and the file as the second."
  @spec upload(
          t(),
          String.t(),
          map(),
          Enumerable.t() | binary(),
          String.t(),
          decoder(),
          call_opts()
        ) ::
          {:ok, term()} | {:error, Error.t()}
  def upload(%__MODULE__{} = transport, path, input, file, filename, decode, opts \\ []) do
    content = if is_binary(file), do: file, else: IO.iodata_to_binary(Enum.to_list(file))

    parts = [
      input: {JSON.encode!(input), content_type: "application/json"},
      file: {content, filename: filename, content_type: "application/octet-stream"}
    ]

    request =
      Req.merge(transport.req,
        method: :post,
        url: transport.base_url <> "/" <> path,
        headers: headers(transport, opts, "application/json"),
        form_multipart: parts
      )
      |> with_timeout(opts)

    case run(request, path) do
      {:ok, %Req.Response{status: status, body: body}} when status in 200..299 ->
        decode_ok(body, decode)

      {:ok, %Req.Response{status: status, body: body}} ->
        {:error, envelope(status, body)}

      {:error, error} ->
        {:error, error}
    end
  end

  defp build(transport, path, method, input, accept, opts) do
    url = transport.base_url <> "/" <> path
    headers = headers(transport, opts, accept)

    base =
      case method do
        :get ->
          Req.merge(transport.req, method: :get, url: url, params: [input: JSON.encode!(input)])

        :post ->
          Req.merge(transport.req, method: :post, url: url, json: input)
      end

    base
    |> Req.merge(headers: headers)
    |> with_timeout(opts)
  end

  defp with_timeout(request, opts) do
    case Keyword.get(opts, :timeout) do
      nil -> request
      timeout -> Req.merge(request, receive_timeout: timeout, connect_options: [timeout: timeout])
    end
  end

  defp headers(transport, opts, accept) do
    static =
      case transport.headers do
        fun when is_function(fun, 0) -> fun.()
        map when is_map(map) -> map
      end

    static
    |> Map.merge(Keyword.get(opts, :headers, %{}))
    |> Map.put("accept", accept)
    |> Enum.map(fn {k, v} -> {to_string(k), to_string(v)} end)
  end

  defp run(request, path) do
    case Req.request(request) do
      {:ok, response} ->
        {:ok, response}

      {:error, %{reason: :timeout}} ->
        {:error,
         %Error{code: :deadline_exceeded, message: "calling #{path} timed out", status: 0}}

      {:error, reason} ->
        {:error,
         %Error{
           code: :unavailable,
           message: "calling #{path}: #{Exception.message(reason)}",
           status: 0
         }}
    end
  rescue
    error in [Req.TransportError] ->
      {:error,
       %Error{
         code: :unavailable,
         message: "calling #{path}: #{Exception.message(error)}",
         status: 0
       }}
  end

  defp decode_ok(body, decode) do
    {:ok, decode.(parse(body))}
  rescue
    error in ArgumentError ->
      {:error,
       %Error{
         code: :internal,
         message: "decoding response: " <> Exception.message(error),
         status: 0
       }}
  end

  defp envelope(status, body), do: Error.from_envelope(status, parse(body))

  defp parse(body) when is_binary(body) do
    case JSON.decode(body) do
      {:ok, value} -> value
      {:error, _} -> body
    end
  end

  defp parse(body), do: body

  defp collect(%Req.Response{body: body}) when is_binary(body), do: body
  defp collect(%Req.Response{body: body}), do: Enum.join(body)

  defp stream_chunks(%Req.Response{body: body}) when is_binary(body), do: [body]
  defp stream_chunks(%Req.Response{body: body}), do: body

  defp stream(response, decode, path) do
    response
    |> stream_chunks()
    |> Stream.transform("", fn chunk, buffer ->
      {events, rest} = SSE.parse(buffer, chunk)
      {events, rest}
    end)
    |> Stream.transform(:open, fn
      _event, :closed ->
        {:halt, :closed}

      :done, _ ->
        {:halt, :closed}

      {:message, data}, state ->
        {[decode.(JSON.decode!(data))], state}

      {:error, data}, _ ->
        error = Error.from_envelope(0, parse(data))
        raise %Error{error | message: "subscription #{path} failed: " <> error.message}
    end)
  end
end
