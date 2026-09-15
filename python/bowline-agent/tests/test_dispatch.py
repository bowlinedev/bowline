from __future__ import annotations

import email.message
import io
import json
import urllib.error
import urllib.request
from typing import Any

from bowline_agent import Call, Dispatcher, RecordingTracer, Result, Tool, tools


class FakeResponse:
    def __init__(self, status: int, body: bytes) -> None:
        self.status = status
        self._body = io.BytesIO(body)

    def read(self) -> bytes:
        return self._body.read()

    def __enter__(self) -> FakeResponse:
        return self

    def __exit__(self, *args: object) -> None:
        return None


class FakeOpener:
    def __init__(self) -> None:
        self.requests: list[urllib.request.Request] = []
        self.responses: list[FakeResponse | Exception] = []

    def __call__(self, request: urllib.request.Request) -> FakeResponse:
        self.requests.append(request)
        next_response = self.responses.pop(0)
        if isinstance(next_response, Exception):
            raise next_response
        return next_response


def http_error(status: int, body: bytes) -> urllib.error.HTTPError:
    return urllib.error.HTTPError(
        "http://ledger/api", status, "error", email.message.Message(), io.BytesIO(body)
    )


def make(ledger: dict[str, Any]) -> tuple[Dispatcher, FakeOpener, list[str]]:
    opener = FakeOpener()
    lines: list[str] = []
    dispatcher = Dispatcher(
        tools(ledger),
        "http://ledger/api/",
        headers={"Authorization": "Bearer dev"},
        tracer=RecordingTracer(lines.append),
        opener=opener,
    )
    return dispatcher, opener, lines


def test_success_posts_json_and_forwards_headers(ledger: dict[str, Any]) -> None:
    dispatcher, opener, lines = make(ledger)
    opener.responses.append(FakeResponse(200, b'{"id":3,"status":"sent"}'))
    result = dispatcher.dispatch(Call(id="1", tool="invoices_get", input={"id": 3}))
    assert result == Result(output={"id": 3, "status": "sent"}, error=None)
    request = opener.requests[0]
    assert request.full_url == "http://ledger/api/invoices.get"
    assert request.get_method() == "POST"
    assert request.data == b'{"id":3}'
    assert request.get_header("Authorization") == "Bearer dev"
    assert request.get_header("Content-type") == "application/json"
    line = json.loads(lines[0])
    assert line["id"] == "1" and line["tool"] == "invoices_get"
    assert line["input"] == {"id": 3} and line["output"] == {"id": 3, "status": "sent"}
    assert line["error"] is None and isinstance(line["durationMs"], int)


def test_error_envelope_becomes_result_error(ledger: dict[str, Any]) -> None:
    dispatcher, opener, lines = make(ledger)
    envelope = {"error": {"code": "NOT_FOUND", "message": "invoice 999 not found"}}
    opener.responses.append(http_error(404, json.dumps(envelope).encode()))
    result = dispatcher.dispatch(Call(id="2", tool="invoices_get", input={"id": 999}))
    assert result.output is None
    assert result.error == {"code": "NOT_FOUND", "message": "invoice 999 not found"}
    assert json.loads(lines[0])["error"]["code"] == "NOT_FOUND"


def test_issues_are_kept(ledger: dict[str, Any]) -> None:
    dispatcher, opener, _ = make(ledger)
    envelope = {
        "error": {
            "code": "INVALID_ARGUMENT",
            "message": "invalid input",
            "issues": [{"path": ["id"], "rule": "required", "message": "is required"}],
        }
    }
    opener.responses.append(http_error(400, json.dumps(envelope).encode()))
    result = dispatcher.dispatch(Call(id="3", tool="invoices_get", input={}))
    assert result.error is not None
    assert result.error["issues"][0]["path"] == ["id"]


def test_non_envelope_error_is_unknown(ledger: dict[str, Any]) -> None:
    dispatcher, opener, _ = make(ledger)
    opener.responses.append(http_error(502, b"<html>bad gateway</html>"))
    result = dispatcher.dispatch(Call(id="4", tool="invoices_list", input={"limit": 2}))
    assert result.error == {"code": "UNKNOWN", "message": "upstream returned HTTP 502"}


def test_connection_failure_is_unavailable(ledger: dict[str, Any]) -> None:
    dispatcher, opener, lines = make(ledger)
    opener.responses.append(urllib.error.URLError("connection refused"))
    result = dispatcher.dispatch(Call(id="5", tool="invoices_list", input={"limit": 2}))
    assert result.error is not None
    assert result.error["code"] == "UNAVAILABLE"
    assert "connection refused" in result.error["message"]
    assert json.loads(lines[0])["error"]["code"] == "UNAVAILABLE"


def test_unknown_tool_never_calls_upstream(ledger: dict[str, Any]) -> None:
    dispatcher, opener, _ = make(ledger)
    result = dispatcher.dispatch(Call(id="6", tool="nope", input={}))
    assert result.error == {"code": "NOT_FOUND", "message": "unknown tool nope"}
    assert opener.requests == []


def test_dispatch_without_tracer() -> None:
    opener = FakeOpener()
    opener.responses.append(FakeResponse(200, b"true"))
    tool = Tool("ping", "ping", "", {}, {}, True, False, ())
    dispatcher = Dispatcher([tool], "http://ledger/api", opener=opener)
    assert dispatcher.dispatch(Call(id="7", tool="ping", input={})) == Result(True, None)
