defmodule BowlineClient.Error do
  @moduledoc "A failed call: the Bowline error code, the message, the HTTP status, details, and validation issues."

  @type code ::
          :canceled
          | :unknown
          | :invalid_argument
          | :deadline_exceeded
          | :not_found
          | :already_exists
          | :permission_denied
          | :resource_exhausted
          | :failed_precondition
          | :aborted
          | :out_of_range
          | :unimplemented
          | :internal
          | :unavailable
          | :data_loss
          | :unauthenticated

  @type t :: %__MODULE__{
          code: code(),
          message: String.t(),
          status: non_neg_integer(),
          type: String.t() | nil,
          details: term(),
          issues: [BowlineClient.Issue.t()]
        }

  defexception code: :unknown, message: "", status: 0, type: nil, details: nil, issues: []

  @codes %{
    "CANCELED" => :canceled,
    "UNKNOWN" => :unknown,
    "INVALID_ARGUMENT" => :invalid_argument,
    "DEADLINE_EXCEEDED" => :deadline_exceeded,
    "NOT_FOUND" => :not_found,
    "ALREADY_EXISTS" => :already_exists,
    "PERMISSION_DENIED" => :permission_denied,
    "RESOURCE_EXHAUSTED" => :resource_exhausted,
    "FAILED_PRECONDITION" => :failed_precondition,
    "ABORTED" => :aborted,
    "OUT_OF_RANGE" => :out_of_range,
    "UNIMPLEMENTED" => :unimplemented,
    "INTERNAL" => :internal,
    "UNAVAILABLE" => :unavailable,
    "DATA_LOSS" => :data_loss,
    "UNAUTHENTICATED" => :unauthenticated
  }

  @spec code_from_string(String.t()) :: code()
  def code_from_string(code), do: Map.get(@codes, code, :unknown)

  @spec code_to_string(code()) :: String.t()
  def code_to_string(code) do
    Enum.find_value(@codes, "UNKNOWN", fn {name, atom} -> if atom == code, do: name end)
  end

  @spec from_envelope(non_neg_integer(), term()) :: t()
  def from_envelope(status, %{"error" => %{"code" => code} = error}) when is_binary(code) do
    %__MODULE__{
      code: code_from_string(code),
      message: to_string(Map.get(error, "message") || ""),
      status: status,
      type: Map.get(error, "type"),
      details: Map.get(error, "details"),
      issues: Enum.map(Map.get(error, "issues") || [], &BowlineClient.Issue.from_map/1)
    }
  end

  def from_envelope(status, _body) do
    %__MODULE__{code: :unknown, message: "HTTP #{status}", status: status}
  end

  @impl true
  def message(%__MODULE__{code: code, message: message, issues: []}) do
    "#{code_to_string(code)}: #{message}"
  end

  def message(%__MODULE__{code: code, message: message, issues: issues}) do
    lines = Enum.map(issues, fn issue -> Enum.join(issue.path, ".") <> ": " <> issue.message end)
    "#{code_to_string(code)}: #{message}\n" <> Enum.join(lines, "\n")
  end
end
