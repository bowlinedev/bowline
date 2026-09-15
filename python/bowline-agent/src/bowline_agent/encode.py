from __future__ import annotations

from typing import Any

from .tools import Tool


def to_anthropic(tools: list[Tool]) -> list[dict[str, Any]]:
    return [
        {"name": t.name, "description": t.description, "input_schema": t.input_schema}
        for t in tools
    ]


def to_openai(tools: list[Tool]) -> list[dict[str, Any]]:
    return [
        {
            "type": "function",
            "function": {
                "name": t.name,
                "description": t.description,
                "parameters": t.input_schema,
            },
        }
        for t in tools
    ]


def to_json_schema(tools: list[Tool]) -> list[dict[str, Any]]:
    return [
        {
            "name": t.name,
            "procedure": t.procedure,
            "description": t.description,
            "inputSchema": t.input_schema,
            "outputSchema": t.output_schema,
            "readOnly": t.read_only,
            "destructive": t.destructive,
            "scopes": sorted(t.scopes),
        }
        for t in tools
    ]
