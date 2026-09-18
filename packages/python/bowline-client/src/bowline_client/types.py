from __future__ import annotations

from enum import Enum
from typing import Annotated, NewType

from pydantic import BaseModel, BeforeValidator, ConfigDict, PlainSerializer


class Method(Enum):
    GET = "GET"
    POST = "POST"
    PUT = "PUT"
    PATCH = "PATCH"
    DELETE = "DELETE"
    HEAD = "HEAD"


class CallOptions(BaseModel):
    headers: dict[str, str] | None = None
    timeout: float | None = None


class Empty(BaseModel):
    model_config = ConfigDict(extra="ignore")


def _parse_big(value: object) -> object:
    if isinstance(value, str):
        return int(value, 10)
    return value


BigInt = Annotated[int, BeforeValidator(_parse_big), PlainSerializer(str, return_type=str)]

DurationNs = NewType("DurationNs", int)
