from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path
from types import ModuleType
from typing import Any

import pytest
from pydantic import TypeAdapter

HERE = Path(__file__).parent
ROWS = sorted(p.name.removesuffix(".golden.py") for p in HERE.glob("*.golden.py"))


def load(row: str) -> ModuleType:
    spec = importlib.util.spec_from_file_location(f"golden_{row}", HERE / f"{row}.golden.py")
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def dump(adapter: TypeAdapter[Any], value: Any) -> Any:
    return adapter.dump_python(value, mode="json", by_alias=True, exclude_unset=True)


@pytest.mark.parametrize("row", ROWS)
def test_golden_round_trips_sample_outputs(row: str) -> None:
    module = load(row)
    samples = json.loads((HERE / f"{row}.samples.json").read_text())
    assert hasattr(module, "Client") and hasattr(module, "SyncClient")
    for procedure, sample in samples.items():
        model = eval(sample["model"], vars(module))
        adapter: TypeAdapter[Any] = TypeAdapter(model)
        first = adapter.validate_python(sample["output"])
        wire = dump(adapter, first)
        second = adapter.validate_python(wire)
        assert dump(adapter, second) == wire, procedure
        assert adapter.validate_json(json.dumps(wire)) == second, procedure
        input_model = eval(sample["input"], vars(module))
        assert isinstance(input_model, type) or hasattr(input_model, "__origin__"), procedure
