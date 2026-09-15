from __future__ import annotations

import re

EMAIL_PATTERN = r"^[^@\s]+@[^@\s]+\.[^@\s]+$"
URL_PATTERN = r"^[A-Za-z][A-Za-z0-9+.-]*://[^\s/?#]+"
UUID_PATTERN = r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"

_email = re.compile(EMAIL_PATTERN)
_url = re.compile(URL_PATTERN)
_uuid = re.compile(UUID_PATTERN)


def is_email(value: str) -> bool:
    return _email.match(value) is not None


def is_url(value: str) -> bool:
    return _url.match(value) is not None


def is_uuid(value: str) -> bool:
    return _uuid.match(value) is not None
