from __future__ import annotations

from collections.abc import Callable
from typing import Any

from bowline_agent import to_anthropic, to_json_schema, to_openai, tools


def test_anthropic_matches_cli_golden(ledger: dict[str, Any], golden: Callable[[str], Any]) -> None:
    assert to_anthropic(tools(ledger)) == golden("anthropic")


def test_openai_matches_cli_golden(ledger: dict[str, Any], golden: Callable[[str], Any]) -> None:
    assert to_openai(tools(ledger)) == golden("openai")


def test_json_schema_matches_cli_golden(
    ledger: dict[str, Any], golden: Callable[[str], Any]
) -> None:
    assert to_json_schema(tools(ledger)) == golden("json-schema")
