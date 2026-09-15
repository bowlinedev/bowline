from __future__ import annotations

from typing import Any

import pytest

from bowline_agent import describe, tools


def test_tools_from_ledger(ledger: dict[str, Any]) -> None:
    result = tools(ledger)
    assert [t.name for t in result] == [
        "customers_search",
        "invoices_get",
        "invoices_list",
        "invoices_void",
    ]
    void = result[3]
    assert void.procedure == "invoices.void"
    assert void.destructive and not void.read_only
    assert void.scopes == ("billing",)
    assert void.description == "Void cancels a draft or sent invoice.\nErrors: InvoiceLocked"
    assert void.input_schema["$ref"] == "#/$defs/VoidInvoiceInput"
    assert "$defs" in void.output_schema
    assert result[0].read_only and result[0].scopes == ("crm",)


def test_scope_and_read_only_filters(ledger: dict[str, Any]) -> None:
    assert [t.name for t in tools(ledger, scopes=["crm"])] == ["customers_search"]
    assert len(tools(ledger, scopes=["billing"])) == 3
    assert tools(ledger, scopes=["nope"]) == []
    assert [t.name for t in tools(ledger, read_only=True)] == [
        "customers_search",
        "invoices_get",
        "invoices_list",
    ]
    assert [t.name for t in tools(ledger, scopes=["billing"], read_only=True)] == [
        "invoices_get",
        "invoices_list",
    ]


def test_missing_schemas_raise() -> None:
    contract = {"procedures": [{"path": "a.b", "tool": {"readOnly": True}}]}
    with pytest.raises(ValueError, match="a.b"):
        tools(contract)


def test_name_collision_raises() -> None:
    contract = {
        "procedures": [
            {"path": "a.b_c", "tool": {}, "schemas": {"input": {}, "output": {}}},
            {"path": "a_b.c", "tool": {}, "schemas": {"input": {}, "output": {}}},
        ]
    }
    with pytest.raises(ValueError, match="a_b_c"):
        tools(contract)


def test_describe_without_doc_lists_errors() -> None:
    contract = {"errors": {"x.A": {"name": "A"}, "x.B": {"name": "B"}}}
    assert describe(contract, {"errors": ["x.A", "x.B"]}) == "Errors: A, B"
    assert describe(contract, {"doc": "  Hello.  "}) == "Hello."
    assert describe(contract, {}) == ""
