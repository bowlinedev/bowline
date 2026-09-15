from __future__ import annotations

import json
from collections.abc import AsyncIterator, Callable, Iterator, Mapping
from typing import Any, BinaryIO, TypeVar

import httpx
from pydantic import BaseModel, TypeAdapter

from .errors import BowlineError, Code
from .sse import Event, aevents, events
from .types import CallOptions, Method

T = TypeVar("T")

HeadersSource = Callable[[], Mapping[str, str]]


def _encode(input: BaseModel) -> dict[str, Any]:
    return input.model_dump(mode="json", exclude_unset=True, by_alias=True)


def _decode(output: type[T], body: bytes) -> T:
    if not body.strip():
        body = b"{}"
    return TypeAdapter(output).validate_json(body)


def _headers(
    static: HeadersSource | None, options: CallOptions | None, accept: str
) -> dict[str, str]:
    merged: dict[str, str] = {}
    if static is not None:
        merged.update(static())
    if options is not None and options.headers:
        merged.update(options.headers)
    merged["accept"] = accept
    return merged


def _request(
    base: str, path: str, method: Method, input: BaseModel
) -> tuple[str, str, dict[str, str] | None, bytes | None]:
    url = f"{base}/{path}"
    payload = json.dumps(_encode(input), separators=(",", ":"))
    if method is Method.GET:
        return "GET", url, {"input": payload}, None
    return "POST", url, None, payload.encode()


def _timeout(options: CallOptions | None) -> httpx.Timeout | httpx._client.UseClientDefault:
    if options is not None and options.timeout is not None:
        return httpx.Timeout(options.timeout)
    return httpx.USE_CLIENT_DEFAULT


def _failure(path: str, exc: Exception) -> BowlineError:
    if isinstance(exc, httpx.TimeoutException):
        return BowlineError(Code.DEADLINE_EXCEEDED, f"timeout calling {path}", 0)
    return BowlineError(Code.UNAVAILABLE, f"network error calling {path}: {exc}", 0)


def _message(event: Event, output: type[T]) -> T | None:
    if event.name == "message":
        return TypeAdapter(output).validate_json(event.data)
    if event.name == "error":
        raise BowlineError.from_envelope(0, event.data)
    return None


def _multipart(input: BaseModel, file: BinaryIO, filename: str) -> list[tuple[str, Any]]:
    payload = json.dumps(_encode(input), separators=(",", ":")).encode()
    return [
        ("input", (None, payload, "application/json")),
        ("file", (filename, file, "application/octet-stream")),
    ]


class Transport:
    def __init__(
        self,
        base_url: str,
        *,
        client: httpx.AsyncClient | None = None,
        headers: HeadersSource | None = None,
    ) -> None:
        self._base = base_url.rstrip("/")
        self._client = client or httpx.AsyncClient()
        self._headers = headers

    async def call(
        self,
        path: str,
        method: Method,
        input: BaseModel,
        output: type[T],
        options: CallOptions | None = None,
    ) -> T:
        verb, url, params, content = _request(self._base, path, method, input)
        headers = _headers(self._headers, options, "application/json")
        if content is not None:
            headers["content-type"] = "application/json"
        try:
            response = await self._client.request(
                verb,
                url,
                params=params,
                content=content,
                headers=headers,
                timeout=_timeout(options),
            )
        except httpx.HTTPError as exc:
            raise _failure(path, exc) from exc
        if response.status_code >= 400:
            raise BowlineError.from_envelope(response.status_code, response.content)
        return _decode(output, response.content)

    async def subscribe(
        self,
        path: str,
        input: BaseModel,
        output: type[T],
        options: CallOptions | None = None,
        method: Method = Method.GET,
    ) -> AsyncIterator[T]:
        verb, url, params, content = _request(self._base, path, method, input)
        headers = _headers(self._headers, options, "text/event-stream")
        if content is not None:
            headers["content-type"] = "application/json"
        try:
            async with self._client.stream(
                verb,
                url,
                params=params,
                content=content,
                headers=headers,
                timeout=_timeout(options),
            ) as response:
                if response.status_code >= 400:
                    body = await response.aread()
                    raise BowlineError.from_envelope(response.status_code, body)
                async for event in aevents(response.aiter_lines()):
                    if event.name == "done":
                        return
                    value = _message(event, output)
                    if value is not None:
                        yield value
        except httpx.HTTPError as exc:
            raise _failure(path, exc) from exc

    async def upload(
        self,
        path: str,
        input: BaseModel,
        file: BinaryIO,
        filename: str,
        output: type[T],
        options: CallOptions | None = None,
    ) -> T:
        headers = _headers(self._headers, options, "application/json")
        try:
            response = await self._client.post(
                f"{self._base}/{path}",
                files=_multipart(input, file, filename),
                headers=headers,
                timeout=_timeout(options),
            )
        except httpx.HTTPError as exc:
            raise _failure(path, exc) from exc
        if response.status_code >= 400:
            raise BowlineError.from_envelope(response.status_code, response.content)
        return _decode(output, response.content)

    async def aclose(self) -> None:
        await self._client.aclose()


class SyncTransport:
    def __init__(
        self,
        base_url: str,
        *,
        client: httpx.Client | None = None,
        headers: HeadersSource | None = None,
    ) -> None:
        self._base = base_url.rstrip("/")
        self._client = client or httpx.Client()
        self._headers = headers

    def call(
        self,
        path: str,
        method: Method,
        input: BaseModel,
        output: type[T],
        options: CallOptions | None = None,
    ) -> T:
        verb, url, params, content = _request(self._base, path, method, input)
        headers = _headers(self._headers, options, "application/json")
        if content is not None:
            headers["content-type"] = "application/json"
        try:
            response = self._client.request(
                verb,
                url,
                params=params,
                content=content,
                headers=headers,
                timeout=_timeout(options),
            )
        except httpx.HTTPError as exc:
            raise _failure(path, exc) from exc
        if response.status_code >= 400:
            raise BowlineError.from_envelope(response.status_code, response.content)
        return _decode(output, response.content)

    def subscribe(
        self,
        path: str,
        input: BaseModel,
        output: type[T],
        options: CallOptions | None = None,
        method: Method = Method.GET,
    ) -> Iterator[T]:
        verb, url, params, content = _request(self._base, path, method, input)
        headers = _headers(self._headers, options, "text/event-stream")
        if content is not None:
            headers["content-type"] = "application/json"
        try:
            with self._client.stream(
                verb,
                url,
                params=params,
                content=content,
                headers=headers,
                timeout=_timeout(options),
            ) as response:
                if response.status_code >= 400:
                    body = response.read()
                    raise BowlineError.from_envelope(response.status_code, body)
                for event in events(response.iter_lines()):
                    if event.name == "done":
                        return
                    value = _message(event, output)
                    if value is not None:
                        yield value
        except httpx.HTTPError as exc:
            raise _failure(path, exc) from exc

    def upload(
        self,
        path: str,
        input: BaseModel,
        file: BinaryIO,
        filename: str,
        output: type[T],
        options: CallOptions | None = None,
    ) -> T:
        headers = _headers(self._headers, options, "application/json")
        try:
            response = self._client.post(
                f"{self._base}/{path}",
                files=_multipart(input, file, filename),
                headers=headers,
                timeout=_timeout(options),
            )
        except httpx.HTTPError as exc:
            raise _failure(path, exc) from exc
        if response.status_code >= 400:
            raise BowlineError.from_envelope(response.status_code, response.content)
        return _decode(output, response.content)

    def close(self) -> None:
        self._client.close()
