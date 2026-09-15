from __future__ import annotations

import io
from datetime import UTC, datetime
from typing import Any

import pytest

from bowline_client import (
    BowlineError,
    CallOptions,
    Code,
    Empty,
    Method,
    SyncTransport,
    Transport,
)

from .conftest import GetInput, Item, Tick


class Size(Empty):
    n: int


async def test_get_encodes_input_and_decodes_output(
    transport: Transport, seen: dict[str, Any]
) -> None:
    item = await transport.call("items.get", Method.GET, GetInput(id=3), Item)
    assert item.id == 3
    assert item.name == "Bearer t"
    assert item.created_at == datetime(2026, 1, 2, 3, 4, 5, tzinfo=UTC)
    assert item.big == 9007199254740993
    assert item.took == 1500
    assert seen["headers"]["accept"] == "application/json"
    assert item.model_dump(mode="json")["big"] == "9007199254740993"


async def test_post_sends_json_and_merges_headers(
    transport: Transport, seen: dict[str, Any]
) -> None:
    options = CallOptions(headers={"x-call": "yes", "authorization": "Bearer override"})
    item = await transport.call("items.create", Method.POST, GetInput(id=4), Item, options)
    assert item.name == "{'id': 4}|application/json|yes"
    assert seen["headers"]["authorization"] == "Bearer override"


async def test_envelope_maps_to_error(transport: Transport) -> None:
    with pytest.raises(BowlineError) as raised:
        await transport.call("items.get", Method.GET, GetInput(id=9), Item)
    err = raised.value
    assert err.code is Code.NOT_FOUND
    assert err.status == 404
    assert err.message == "item 9 not found"
    assert err.type == "Gone"
    assert err.issues[0].path == ["id"]
    assert err.details_as(GetInput).id == 9


async def test_non_envelope_failure_is_unknown(transport: Transport) -> None:
    with pytest.raises(BowlineError) as raised:
        await transport.call("plain", Method.POST, Empty(), Empty)
    assert raised.value.code is Code.UNKNOWN
    assert raised.value.status == 502


async def test_network_failure_is_unavailable(transport: Transport) -> None:
    with pytest.raises(BowlineError) as raised:
        await transport.call("down", Method.GET, Empty(), Empty)
    assert raised.value.code is Code.UNAVAILABLE
    assert raised.value.status == 0


async def test_timeout_is_deadline_exceeded(transport: Transport) -> None:
    with pytest.raises(BowlineError) as raised:
        await transport.call("slow", Method.GET, Empty(), Empty, CallOptions(timeout=0.01))
    assert raised.value.code is Code.DEADLINE_EXCEEDED


async def test_unknown_code_maps_to_unknown() -> None:
    err = BowlineError.from_envelope(418, b'{"error":{"code":"TEAPOT","message":"short"}}')
    assert err.code is Code.UNKNOWN
    assert err.message == "short"


async def test_subscribe_yields_messages_until_done(transport: Transport) -> None:
    ticks = [tick.n async for tick in transport.subscribe("ticks", Empty(), Tick)]
    assert ticks == [1, 2, 3]


async def test_subscribe_can_post_sensitive_inputs(transport: Transport) -> None:
    ticks = [t.n async for t in transport.subscribe("secret", Empty(), Tick, method=Method.POST)]
    assert ticks == [1, 2, 3]


async def test_subscribe_raises_on_error_event(transport: Transport) -> None:
    seen: list[int] = []
    with pytest.raises(BowlineError) as raised:
        async for tick in transport.subscribe("broken", Empty(), Tick):
            seen.append(tick.n)
    assert seen == [1]
    assert raised.value.code is Code.PERMISSION_DENIED


async def test_subscribe_rejected_before_streaming(transport: Transport) -> None:
    with pytest.raises(BowlineError) as raised:
        async for _ in transport.subscribe("denied", Empty(), Tick):
            pass
    assert raised.value.status == 403


async def test_upload_sends_two_parts(transport: Transport) -> None:
    size = await transport.upload("attach", GetInput(id=1), io.BytesIO(b"hello"), "r.bin", Size)
    assert size.n == 5


def test_sync_transport_mirrors_async(sync_transport: SyncTransport) -> None:
    item = sync_transport.call("items.get", Method.GET, GetInput(id=3), Item)
    assert item.id == 3
    with pytest.raises(BowlineError) as raised:
        sync_transport.call("items.get", Method.GET, GetInput(id=9), Item)
    assert raised.value.code is Code.NOT_FOUND
    assert [t.n for t in sync_transport.subscribe("ticks", Empty(), Tick)] == [1, 2, 3]
    size = sync_transport.upload("attach", GetInput(id=1), io.BytesIO(b"hello"), "r.bin", Size)
    assert size.n == 5
    with pytest.raises(BowlineError) as raised:
        sync_transport.call("down", Method.GET, Empty(), Empty)
    assert raised.value.code is Code.UNAVAILABLE
    with pytest.raises(BowlineError) as raised:
        sync_transport.call("slow", Method.GET, Empty(), Empty, CallOptions(timeout=0.01))
    assert raised.value.code is Code.DEADLINE_EXCEEDED
    with pytest.raises(BowlineError) as raised:
        for _ in sync_transport.subscribe("broken", Empty(), Tick):
            pass
    assert raised.value.code is Code.PERMISSION_DENIED
