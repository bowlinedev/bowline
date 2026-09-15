from __future__ import annotations

from collections.abc import AsyncIterator, Iterable, Iterator


class Event:
    __slots__ = ("data", "name")

    def __init__(self, name: str, data: str) -> None:
        self.name = name
        self.data = data


class Parser:
    def __init__(self) -> None:
        self._name = ""
        self._data: list[str] = []

    def feed(self, line: str) -> Event | None:
        line = line.rstrip("\r\n")
        if line == "":
            if self._name == "" and not self._data:
                return None
            event = Event(self._name or "message", "\n".join(self._data))
            self._name = ""
            self._data = []
            return event
        if line.startswith(":"):
            return None
        field, _, value = line.partition(":")
        value = value.removeprefix(" ")
        if field == "event":
            self._name = value
        elif field == "data":
            self._data.append(value)
        return None


def events(lines: Iterable[str]) -> Iterator[Event]:
    parser = Parser()
    for line in lines:
        event = parser.feed(line)
        if event is not None:
            yield event


async def aevents(lines: AsyncIterator[str]) -> AsyncIterator[Event]:
    parser = Parser()
    async for line in lines:
        event = parser.feed(line)
        if event is not None:
            yield event
