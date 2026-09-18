from __future__ import annotations

import json
from datetime import datetime
from typing import Any

import httpx
import pytest
from pydantic import BaseModel, ConfigDict

from bowline_client import BigInt, DurationNs, SyncTransport, Transport


class GetInput(BaseModel):
    model_config = ConfigDict(extra="ignore")

    id: int
    note: str | None = None


class Item(BaseModel):
    model_config = ConfigDict(extra="ignore")

    id: int
    name: str
    created_at: datetime
    big: BigInt
    took: DurationNs


class Tick(BaseModel):
    n: int


class Echo(BaseModel):
    model_config = ConfigDict(extra="ignore")

    method: str
    path: str
    query: str
    body: str


ENVELOPE = {
    "error": {
        "code": "NOT_FOUND",
        "message": "item 9 not found",
        "type": "Gone",
        "details": {"id": 9},
        "issues": [{"path": ["id"], "rule": "exists", "message": "missing"}],
    }
}

SSE = (
    ": open\n\n"
    'event: message\ndata: {"n":1}\n\n'
    ": ping\n\n"
    'event: message\ndata: {"n":2}\n\n'
    'event: message\ndata: {"n":3}\n\n'
    "event: done\ndata: {}\n\n"
)

SSE_ERROR = (
    'event: message\ndata: {"n":1}\n\n'
    'event: error\ndata: {"error":{"code":"PERMISSION_DENIED","message":"nope"}}\n\n'
)


def handler(request: httpx.Request) -> httpx.Response:
    if request.url.path.startswith("/v1/invoices"):
        return httpx.Response(
            200,
            json={
                "method": request.method,
                "path": request.url.path,
                "query": request.url.query.decode(),
                "body": request.content.decode(),
            },
        )
    path = request.url.path.rsplit("/", 1)[-1]
    if path == "items.get":
        if request.method != "GET":
            return httpx.Response(405)
        input = json.loads(request.url.params["input"])
        if input["id"] == 9:
            return httpx.Response(404, json=ENVELOPE)
        return httpx.Response(
            200,
            json={
                "id": input["id"],
                "name": request.headers.get("authorization", "anonymous"),
                "created_at": "2026-01-02T03:04:05Z",
                "big": "9007199254740993",
                "took": 1500,
                "extra": True,
            },
        )
    if path == "items.create":
        body = json.loads(request.content)
        return httpx.Response(
            200,
            json={
                "id": 1,
                "name": f"{body}|{request.headers['content-type']}|{request.headers['x-call']}",
                "created_at": "2026-01-02T03:04:05+00:00",
                "big": 7,
                "took": 1,
            },
        )
    if path == "plain":
        return httpx.Response(502, text="bad gateway")
    if path == "ticks":
        assert request.headers["accept"] == "text/event-stream"
        assert request.method == "GET"
        return httpx.Response(200, text=SSE, headers={"content-type": "text/event-stream"})
    if path == "secret":
        assert request.method == "POST" and json.loads(request.content) == {}
        return httpx.Response(200, text=SSE, headers={"content-type": "text/event-stream"})
    if path == "broken":
        return httpx.Response(200, text=SSE_ERROR, headers={"content-type": "text/event-stream"})
    if path == "denied":
        return httpx.Response(403, json={"error": {"code": "PERMISSION_DENIED", "message": "no"}})
    if path == "attach":
        content = request.content
        first = content.index(b'name="input"')
        second = content.index(b'name="file"')
        assert first < second
        assert b'filename="r.bin"' in content
        assert b"hello" in content
        return httpx.Response(200, json={"n": len(b"hello")})
    if path == "slow":
        raise httpx.ReadTimeout("slow")
    if path == "down":
        raise httpx.ConnectError("refused")
    return httpx.Response(404, json={"error": {"code": "UNIMPLEMENTED", "message": path}})


def _record(store: dict[str, Any]) -> Any:
    def wrapped(request: httpx.Request) -> httpx.Response:
        store["headers"] = dict(request.headers)
        return handler(request)

    return wrapped


@pytest.fixture
def seen() -> dict[str, Any]:
    return {}


@pytest.fixture
def transport(seen: dict[str, Any]) -> Transport:
    client = httpx.AsyncClient(transport=httpx.MockTransport(_record(seen)))
    return Transport("http://api/v1/", client=client, headers=lambda: {"authorization": "Bearer t"})


@pytest.fixture
def sync_transport(seen: dict[str, Any]) -> SyncTransport:
    client = httpx.Client(transport=httpx.MockTransport(_record(seen)))
    return SyncTransport(
        "http://api/v1", client=client, headers=lambda: {"authorization": "Bearer t"}
    )
