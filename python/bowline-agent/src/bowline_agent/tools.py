from __future__ import annotations

import json
import os
from collections.abc import Iterable
from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class Tool:
    name: str
    procedure: str
    description: str
    input_schema: dict[str, Any]
    output_schema: dict[str, Any]
    read_only: bool
    destructive: bool
    scopes: tuple[str, ...]


def load_contract(path: str | os.PathLike[str]) -> dict[str, Any]:
    with open(path, encoding="utf-8") as f:
        document = json.load(f)
    if not isinstance(document, dict):
        raise ValueError(f"{os.fspath(path)}: contract must be a JSON object")
    return document


def tools(
    contract: dict[str, Any],
    *,
    scopes: Iterable[str] | None = None,
    read_only: bool = False,
) -> list[Tool]:
    wanted = set(scopes) if scopes is not None else None
    result: list[Tool] = []
    seen: dict[str, str] = {}
    for procedure in contract.get("procedures", []):
        tool = procedure.get("tool")
        if tool is None:
            continue
        declared = tuple(tool.get("scopes") or ())
        if read_only and not tool.get("readOnly", False):
            continue
        if wanted and not wanted.intersection(declared):
            continue
        path = procedure["path"]
        name = path.replace(".", "_")
        if name in seen:
            raise ValueError(f"tool name {name!r} is used by both {seen[name]} and {path}")
        seen[name] = path
        schemas = procedure.get("schemas")
        if not schemas:
            raise ValueError(
                f"procedure {path} has no embedded schemas; "
                'regenerate the contract with "schemas": true'
            )
        result.append(
            Tool(
                name=name,
                procedure=path,
                description=describe(contract, procedure),
                input_schema=schemas["input"],
                output_schema=schemas["output"],
                read_only=bool(tool.get("readOnly", False)),
                destructive=bool(tool.get("destructive", False)),
                scopes=declared,
            )
        )
    return result


def describe(contract: dict[str, Any], procedure: dict[str, Any]) -> str:
    description = (procedure.get("doc") or "").strip()
    error_ids = procedure.get("errors") or []
    if error_ids:
        declared = contract.get("errors") or {}
        names = [declared[i]["name"] for i in error_ids if i in declared]
        if description:
            description += "\n"
        description += "Errors: " + ", ".join(names)
    return description
