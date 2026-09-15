from __future__ import annotations

import json
import time
import urllib.error
import urllib.request
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from typing import Any, Protocol

from .tools import Tool


@dataclass(frozen=True)
class Call:
    id: str
    tool: str
    input: dict[str, Any]


@dataclass(frozen=True)
class Result:
    output: Any | None
    error: dict[str, Any] | None


class Tracer(Protocol):
    def start(self, call: Call) -> Callable[[Result], None]: ...


class RecordingTracer:
    def __init__(self, write: Callable[[str], None]) -> None:
        self._write = write

    def start(self, call: Call) -> Callable[[Result], None]:
        started = time.monotonic()

        def finish(result: Result) -> None:
            line = {
                "id": call.id,
                "tool": call.tool,
                "input": call.input,
                "output": result.output,
                "error": result.error,
                "durationMs": int((time.monotonic() - started) * 1000),
            }
            self._write(json.dumps(line, separators=(",", ":"), sort_keys=True) + "\n")

        return finish


class Dispatcher:
    def __init__(
        self,
        tools: list[Tool],
        url: str,
        *,
        headers: Mapping[str, str] | None = None,
        tracer: Tracer | None = None,
        opener: Callable[..., Any] | None = None,
    ) -> None:
        self._tools = {t.name: t for t in tools}
        self._url = url.rstrip("/")
        self._headers = dict(headers or {})
        self._tracer = tracer
        self._opener = opener or urllib.request.urlopen

    def dispatch(self, call: Call) -> Result:
        finish = self._tracer.start(call) if self._tracer is not None else None
        result = self._execute(call)
        if finish is not None:
            finish(result)
        return result

    def _execute(self, call: Call) -> Result:
        tool = self._tools.get(call.tool)
        if tool is None:
            return Result(output=None, error=_error("NOT_FOUND", f"unknown tool {call.tool}"))
        request = self._request(tool, call.input)
        try:
            with self._opener(request) as response:
                body = response.read()
        except urllib.error.HTTPError as exc:
            envelope = _envelope(exc.read())
            if envelope is None:
                envelope = _error("UNKNOWN", f"upstream returned HTTP {exc.code}")
            return Result(output=None, error=envelope)
        except (urllib.error.URLError, OSError) as exc:
            return Result(output=None, error=_error("UNAVAILABLE", str(exc)))
        return Result(output=json.loads(body) if body else None, error=None)

    def _request(self, tool: Tool, payload: dict[str, Any]) -> urllib.request.Request:
        encoded = json.dumps(payload, separators=(",", ":"))
        headers = {"Accept": "application/json", **self._headers}
        headers["Content-Type"] = "application/json"
        return urllib.request.Request(
            f"{self._url}/{tool.procedure}",
            data=encoded.encode("utf-8"),
            headers=headers,
            method="POST",
        )


def _error(code: str, message: str) -> dict[str, Any]:
    return {"code": code, "message": message}


def _envelope(body: bytes) -> dict[str, Any] | None:
    try:
        parsed = json.loads(body)
    except ValueError:
        return None
    if isinstance(parsed, dict):
        error = parsed.get("error")
        if isinstance(error, dict) and isinstance(error.get("code"), str):
            return error
    return None
