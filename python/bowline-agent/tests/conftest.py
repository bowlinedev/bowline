from __future__ import annotations

import json
from collections.abc import Callable
from pathlib import Path
from typing import Any

import pytest

from bowline_agent import load_contract

ROOT = Path(__file__).resolve().parents[3]
GOLDENS = ROOT / "cmd" / "bowline" / "internal" / "tools" / "testdata"


@pytest.fixture
def ledger() -> dict[str, Any]:
    return load_contract(ROOT / "examples" / "ledger" / "api" / "bowline.contract.json")


@pytest.fixture
def golden() -> Callable[[str], Any]:
    def load(name: str) -> Any:
        with open(GOLDENS / f"ledger.{name}.golden.json", encoding="utf-8") as f:
            return json.load(f)

    return load
