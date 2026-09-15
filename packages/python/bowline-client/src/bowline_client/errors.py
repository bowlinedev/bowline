from __future__ import annotations

from enum import StrEnum
from typing import Any, TypeVar

from pydantic import BaseModel, TypeAdapter

T = TypeVar("T")


class Code(StrEnum):
    CANCELED = "CANCELED"
    UNKNOWN = "UNKNOWN"
    INVALID_ARGUMENT = "INVALID_ARGUMENT"
    DEADLINE_EXCEEDED = "DEADLINE_EXCEEDED"
    NOT_FOUND = "NOT_FOUND"
    ALREADY_EXISTS = "ALREADY_EXISTS"
    PERMISSION_DENIED = "PERMISSION_DENIED"
    RESOURCE_EXHAUSTED = "RESOURCE_EXHAUSTED"
    FAILED_PRECONDITION = "FAILED_PRECONDITION"
    ABORTED = "ABORTED"
    OUT_OF_RANGE = "OUT_OF_RANGE"
    UNIMPLEMENTED = "UNIMPLEMENTED"
    INTERNAL = "INTERNAL"
    UNAVAILABLE = "UNAVAILABLE"
    DATA_LOSS = "DATA_LOSS"
    UNAUTHENTICATED = "UNAUTHENTICATED"

    @classmethod
    def _missing_(cls, value: object) -> Code:
        return cls.UNKNOWN


class Issue(BaseModel):
    path: list[str]
    rule: str
    message: str


class BowlineError(Exception):
    def __init__(
        self,
        code: Code,
        message: str,
        status: int = 0,
        *,
        details: object | None = None,
        issues: list[Issue] | None = None,
        type: str | None = None,
    ) -> None:
        super().__init__(f"{code.value}: {message}")
        self.code = code
        self.message = message
        self.status = status
        self.details = details
        self.issues = issues or []
        self.type = type

    def details_as(self, model: type[T]) -> T:
        return TypeAdapter(model).validate_python(self.details)

    @classmethod
    def from_envelope(cls, status: int, body: bytes | str) -> BowlineError:
        try:
            envelope = TypeAdapter(_Envelope).validate_json(body)
        except ValueError:
            return cls(Code.UNKNOWN, f"HTTP {status}", status)
        wire = envelope.error
        return cls(
            Code(wire.code),
            wire.message,
            status,
            details=wire.details,
            issues=wire.issues,
            type=wire.type,
        )


class _WireError(BaseModel):
    code: str
    message: str
    type: str | None = None
    details: Any = None
    issues: list[Issue] = []


class _Envelope(BaseModel):
    error: _WireError
