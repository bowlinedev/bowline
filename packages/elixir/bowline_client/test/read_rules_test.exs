defmodule BowlineClient.ReadRulesTest do
  use ExUnit.Case, async: true

  alias BowlineClient.Issue
  alias BowlineClient.Read
  alias BowlineClient.Rules
  alias BowlineClient.SSE

  test "reads every primitive shape" do
    map = %{
      "n" => 3,
      "big" => "9007199254740993",
      "f" => 1,
      "b" => true,
      "s" => "x",
      "t" => "2026-01-02T03:04:05Z",
      "tf" => "2026-01-02T03:04:05.250+02:00",
      "bytes" => Base.encode64("hi"),
      "list" => [1, 2],
      "map" => %{"a" => 1},
      "raw" => %{"any" => [1]}
    }

    assert Read.int(map, "n") == 3
    assert Read.big_int(map, "big") == 9_007_199_254_740_993
    assert Read.big_int(%{"big" => 7}, "big") == 7
    assert Read.float(map, "f") == 1.0
    assert Read.bool(map, "b") == true
    assert Read.string(map, "s") == "x"
    assert Read.timestamp(map, "t") == ~U[2026-01-02 03:04:05Z]
    assert Read.timestamp(map, "tf") == ~U[2026-01-02 01:04:05.250Z]
    assert Read.bytes(map, "bytes") == "hi"
    assert Read.list(map, "list", &(&1 * 2)) == [2, 4]
    assert Read.map(map, "map", &(&1 + 1)) == %{"a" => 2}
    assert Read.raw(map, "raw") == %{"any" => [1]}
    assert Read.optional(map, "missing", & &1) == nil
    assert Read.optional(map, "n", &(&1 + 1)) == 4
    assert Read.int(%{}, "n") == 0
    assert Read.string(%{}, "s") == ""
  end

  test "shape mismatches name the key" do
    assert_raise ArgumentError, ~r/^id: expected integer/, fn ->
      Read.int(%{"id" => "x"}, "id")
    end

    assert_raise ArgumentError, ~r/^at: expected RFC 3339/, fn ->
      Read.timestamp(%{"at" => "yesterday"}, "at")
    end

    assert_raise ArgumentError, ~r/^blob: expected base64/, fn ->
      Read.bytes(%{"blob" => "!!"}, "blob")
    end

    assert_raise ArgumentError, ~r/^big: expected decimal/, fn ->
      Read.big_int(%{"big" => "1x"}, "big")
    end
  end

  test "prefixed prepends the path" do
    assert Read.prefixed(["lines", "0"], [Issue.new(["qty"], "min", "m")]) ==
             [Issue.new(["lines", "0", "qty"], "min", "m")]
  end

  test "rules produce the server's messages" do
    assert Rules.required(["name"], "") == [Issue.new(["name"], "required", "is required")]
    assert Rules.required(["n"], 0) == [Issue.new(["n"], "required", "is required")]
    assert Rules.required(["n"], 1) == []
    assert Rules.min(["s"], "ab", 3) == [Issue.new(["s"], "min", "must be at least 3 characters")]
    assert Rules.min(["l"], [], 1) == [Issue.new(["l"], "min", "must be at least 1 items")]
    assert Rules.max(["n"], 5, 4) == [Issue.new(["n"], "max", "must be at most 4")]
    assert Rules.len(["l"], [1], 2) == [Issue.new(["l"], "len", "must be exactly 2 items")]

    assert Rules.one_of(["k"], "other", ["draft", "final"]) == [
             Issue.new(["k"], "oneof", "must be one of draft final")
           ]

    assert Rules.one_of(["k"], 2, ["1", "2"]) == []

    assert Rules.email(["e"], "nope") == [
             Issue.new(["e"], "email", "must be a valid email address")
           ]

    assert Rules.email(["e"], "a@b.co") == []
    assert Rules.url(["u"], "nope") == [Issue.new(["u"], "url", "must be a valid URL")]
    assert Rules.url(["u"], "https://example.com/x") == []
    assert Rules.uuid(["u"], "nope") == [Issue.new(["u"], "uuid", "must be a valid UUID")]
    assert Rules.uuid(["u"], "123e4567-e89b-42d3-a456-426614174000") == []
  end

  test "SSE parsing handles comments, split frames, and event names" do
    {events, rest} = SSE.parse("", ": open\n\nevent: message\ndata: {\"n\":1}\n\nevent: mess")
    assert events == [{:message, ~s({"n":1})}]
    {events, rest} = SSE.parse(rest, "age\ndata: {\"n\":2}\n\nevent: done\ndata: {}\n\n")
    assert events == [{:message, ~s({"n":2})}, :done]
    assert rest == ""
  end
end
