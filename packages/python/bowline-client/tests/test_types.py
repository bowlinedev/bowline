from __future__ import annotations

from datetime import UTC, datetime

import pytest
from pydantic import Base64Bytes, BaseModel, ValidationError

from bowline_client import BigInt, DurationNs, Empty, is_email, is_url, is_uuid
from bowline_client.sse import events


class Sample(BaseModel):
    big: BigInt
    took: DurationNs
    blob: Base64Bytes
    at: datetime


def test_bigint_round_trips_through_strings() -> None:
    sample = Sample.model_validate(
        {"big": "18446744073709551615", "took": 5, "blob": "aGVsbG8=", "at": "2026-01-01T00:00:00Z"}
    )
    assert sample.big == 18446744073709551615
    assert sample.blob == b"hello"
    assert sample.at == datetime(2026, 1, 1, tzinfo=UTC)
    dumped = sample.model_dump(mode="json")
    assert dumped == {
        "big": "18446744073709551615",
        "took": 5,
        "blob": "aGVsbG8=",
        "at": "2026-01-01T00:00:00Z",
    }
    assert Sample.model_validate({**dumped, "big": 12}).big == 12
    with pytest.raises(ValidationError):
        Sample.model_validate({**dumped, "big": "twelve"})


def test_timestamps_are_aware() -> None:
    sample = Sample.model_validate(
        {"big": 1, "took": 1, "blob": "", "at": "2026-01-01T00:00:00.123456789+02:00"}
    )
    assert sample.at.tzinfo is not None
    assert sample.at.utcoffset() is not None


def test_empty_ignores_extra_fields() -> None:
    assert Empty.model_validate({"anything": 1}).model_dump() == {}


def test_rules() -> None:
    assert is_email("ada@example.com") and not is_email("nope")
    assert is_url("https://example.com/x") and not is_url("example.com")
    assert is_uuid("123e4567-e89b-42d3-a456-426614174000") and not is_uuid("nope")


def test_sse_parser_handles_comments_and_multiline_data() -> None:
    lines = [": open", "", "event: message", 'data: {"a":', "data: 1}", "", "data: bare", ""]
    parsed = [(e.name, e.data) for e in events(lines)]
    assert parsed == [("message", '{"a":\n1}'), ("message", "bare")]
