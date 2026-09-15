defmodule BowlineClient.TransportTest do
  use ExUnit.Case, async: true

  alias BowlineClient.Error
  alias BowlineClient.TestServer
  alias BowlineClient.Transport

  defp serve(handler) do
    server = {url, _, _} = TestServer.start(handler)
    on_exit(fn -> TestServer.stop(server) end)
    url
  end

  test "GET queries carry the input as a query parameter and static plus per-call headers" do
    url = serve(fn _ -> {200, ~s({"id":3,"name":"Ada"})} end)

    transport =
      Transport.new(base_url: url <> "/api/", headers: fn -> %{"authorization" => "Bearer t"} end)

    assert {:ok, %{"id" => 3}} =
             Transport.call(transport, "users.get", :get, %{"id" => 3}, & &1,
               headers: %{"x-tenant" => "acme"}
             )

    assert_receive {:request, request}
    assert request.method == "GET"
    assert request.path == "/api/users.get"
    assert request.query == %{"input" => ~s({"id":3})}
    assert request.headers["authorization"] == "Bearer t"
    assert request.headers["x-tenant"] == "acme"
    assert request.headers["accept"] == "application/json"
  end

  test "POST mutations send a JSON body" do
    url = serve(fn _ -> {200, ~s({"ok":true})} end)
    transport = Transport.new(base_url: url)

    assert {:ok, %{"ok" => true}} =
             Transport.call(transport, "users.create", :post, %{"name" => "x"}, & &1)

    assert_receive {:request, request}
    assert request.method == "POST"
    assert request.headers["content-type"] == "application/json"
    assert JSON.decode!(request.body) == %{"name" => "x"}
  end

  test "an error envelope becomes a BowlineClient.Error with code, status, details, and issues" do
    body =
      ~s({"error":{"code":"INVALID_ARGUMENT","message":"invalid input","issues":[{"path":["lines","0","quantity"],"rule":"min","message":"must be at least 1"}]}})

    url = serve(fn _ -> {400, body} end)
    transport = Transport.new(base_url: url)
    assert {:error, %Error{} = error} = Transport.call(transport, "x", :post, %{}, & &1)
    assert error.code == :invalid_argument
    assert error.status == 400
    assert error.message == "invalid input"
    assert [%BowlineClient.Issue{path: ["lines", "0", "quantity"], rule: "min"}] = error.issues
    assert Exception.message(error) =~ "lines.0.quantity: must be at least 1"
  end

  test "a non-envelope failure maps to unknown with the status" do
    url = serve(fn _ -> {502, [{"content-type", "text/html"}], "<h1>bad gateway</h1>"} end)
    transport = Transport.new(base_url: url)

    assert {:error, %Error{code: :unknown, status: 502}} =
             Transport.call(transport, "x", :get, %{}, & &1)
  end

  test "a network failure maps to unavailable with status 0" do
    {url, _, listen} = TestServer.start(fn _ -> {200, "{}"} end)
    :gen_tcp.close(listen)
    transport = Transport.new(base_url: url)

    assert {:error, %Error{code: :unavailable, status: 0}} =
             Transport.call(transport, "x", :get, %{}, & &1)
  end

  test "a timeout maps to deadline_exceeded" do
    url = serve(fn _ -> {:delay, 300, {200, "{}"}} end)
    transport = Transport.new(base_url: url)

    assert {:error, %Error{code: :deadline_exceeded}} =
             Transport.call(transport, "x", :get, %{}, & &1, timeout: 50)
  end

  test "decoder failures surface as internal errors instead of raising" do
    url = serve(fn _ -> {200, ~s({"id":"three"})} end)
    transport = Transport.new(base_url: url)

    assert {:error, %Error{code: :internal, message: message}} =
             Transport.call(transport, "x", :get, %{}, &BowlineClient.Read.int(&1, "id"))

    assert message =~ "id: expected integer"
  end

  test "subscriptions stream decoded messages until done" do
    chunks = [
      ": open\n\n",
      "event: message\ndata: {\"n\":1}\n\n",
      ": ping\n\nevent: message\ndata: {\"n\":2}\n\nevent: mess",
      "age\ndata: {\"n\":3}\n\nevent: done\ndata: {}\n\n"
    ]

    url = serve(fn _ -> {:stream, chunks} end)
    transport = Transport.new(base_url: url)
    assert {:ok, stream} = Transport.subscribe(transport, "ticks", %{"count" => 3}, & &1["n"])
    assert Enum.to_list(stream) == [1, 2, 3]
    assert_receive {:request, request}
    assert request.headers["accept"] == "text/event-stream"
    assert request.query == %{"input" => ~s({"count":3})}
  end

  test "a subscription error event raises the envelope" do
    chunks = ["event: error\ndata: {\"error\":{\"code\":\"NOT_FOUND\",\"message\":\"gone\"}}\n\n"]
    url = serve(fn _ -> {:stream, chunks} end)
    transport = Transport.new(base_url: url)
    assert {:ok, stream} = Transport.subscribe(transport, "ticks", %{}, & &1)
    assert_raise Error, fn -> Enum.to_list(stream) end
  end

  test "uploads send the input part before the file part" do
    url = serve(fn _ -> {200, ~s({"size":5})} end)
    transport = Transport.new(base_url: url)

    assert {:ok, %{"size" => 5}} =
             Transport.upload(
               transport,
               "files.put",
               %{"label" => "x"},
               ["hel", "lo"],
               "a.bin",
               & &1
             )

    assert_receive {:request, request}
    assert request.headers["content-type"] =~ "multipart/form-data"
    [_, input_part, file_part | _] = String.split(request.body, "--")
    assert input_part =~ ~s(name="input")
    assert input_part =~ ~s({"label":"x"})
    assert file_part =~ ~s(name="file")
    assert file_part =~ ~s(filename="a.bin")
    assert file_part =~ "hello"
  end
end
