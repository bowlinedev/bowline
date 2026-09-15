from __future__ import annotations

import asyncio
import os
import socket
import subprocess
import time
from collections.abc import Iterator
from datetime import UTC
from pathlib import Path

import httpx
import pytest
from bowline_client import BowlineError, Code, SyncTransport, Transport
from pydantic import ValidationError

from bowline import (
    Client,
    CreateInvoiceInput,
    GetInvoiceInput,
    InvoiceLocked,
    Line,
    ListInvoicesInput,
    Status,
    SyncClient,
    VoidInvoiceInput,
    WatchInput,
)
from ledger_cli import run

LEDGER = Path(__file__).resolve().parent.parent


@pytest.fixture(scope="session")
def server() -> Iterator[str]:
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        port = probe.getsockname()[1]
    env = {**os.environ, "ADDR": f"127.0.0.1:{port}", "LEDGER_FIXED_TIME": "2026-09-15T12:00:00Z"}
    process = subprocess.Popen(
        ["go", "run", "./cmd/server"],
        cwd=LEDGER,
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    url = f"http://127.0.0.1:{port}/api"
    deadline = time.monotonic() + 120
    try:
        while True:
            try:
                if httpx.get(f"{url}/health", timeout=1).status_code == 200:
                    break
            except httpx.HTTPError:
                pass
            if time.monotonic() > deadline or process.poll() is not None:
                raise RuntimeError("ledger server did not start")
            time.sleep(0.2)
        yield url
    finally:
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
        subprocess.run(["pkill", "-f", f"ADDR=127.0.0.1:{port}"], check=False)


@pytest.fixture
def client(server: str) -> SyncClient:
    return SyncClient(SyncTransport(server))


def test_list_returns_seeded_invoices(client: SyncClient) -> None:
    page = client.invoices.list(ListInvoicesInput(limit=10))
    ids = sorted(invoice.id for invoice in page.items)
    assert ids[:2] == [3, 4]
    third = next(invoice for invoice in page.items if invoice.id == 3)
    assert third.status is Status.StatusSent
    assert third.total == "USD 1500.00"
    assert third.createdAt.tzinfo is not None
    assert third.createdAt.astimezone(UTC).isoformat() == "2026-09-15T12:00:00+00:00"


def test_server_validation_issues_surface(client: SyncClient) -> None:
    bad = CreateInvoiceInput.model_construct(
        customerId=1,
        lines=[Line.model_construct(description="", quantity=0, unitPrice="USD 1.00")],
    )
    with pytest.raises(BowlineError) as raised:
        client.invoices.create(bad)
    err = raised.value
    assert err.code is Code.INVALID_ARGUMENT
    assert err.status == 400
    assert [".".join(issue.path) for issue in err.issues] == [
        "lines.0.description",
        "lines.0.quantity",
    ]


def test_client_validation_runs_before_sending() -> None:
    with pytest.raises(ValidationError):
        Line(description="", quantity=0, unitPrice="USD 1.00")


def test_create_and_get(client: SyncClient) -> None:
    created = client.invoices.create(
        CreateInvoiceInput(
            customerId=1,
            lines=[Line(description="Widgets", quantity=3, unitPrice="USD 10.00")],
        )
    )
    assert created.status is Status.StatusDraft
    assert created.total == "USD 30.00"
    fetched = client.invoices.get(GetInvoiceInput(id=created.id))
    assert fetched.model_dump(mode="json") == created.model_dump(mode="json")


def test_void_paid_invoice_is_locked(client: SyncClient) -> None:
    with pytest.raises(BowlineError) as raised:
        client.invoices.void(VoidInvoiceInput(id=4))
    err = raised.value
    assert err.code is Code.FAILED_PRECONDITION
    assert err.status == 412
    assert err.type == "InvoiceLocked"
    locked = err.details_as(InvoiceLocked)
    assert locked.id == 4 and locked.status is Status.StatusPaid


def test_not_found(client: SyncClient) -> None:
    with pytest.raises(BowlineError) as raised:
        client.invoices.get(GetInvoiceInput(id=999))
    assert raised.value.code is Code.NOT_FOUND


def test_subscription_receives_a_create(server: str) -> None:
    async def scenario() -> list[int]:
        transport = Transport(server)
        client = Client(transport)
        seen: list[int] = []
        stream = client.invoices.watch(WatchInput())
        ready = asyncio.Event()

        async def listen() -> None:
            ready.set()
            async for invoice in stream:
                seen.append(invoice.id)
                if len(seen) == 1:
                    break

        task = asyncio.create_task(listen())
        await ready.wait()
        await asyncio.sleep(0.3)
        created = await client.invoices.create(
            CreateInvoiceInput(
                customerId=2,
                lines=[Line(description="Stream", quantity=1, unitPrice="USD 5.00")],
            )
        )
        await asyncio.wait_for(task, timeout=10)
        await transport.aclose()
        assert seen == [created.id]
        return seen

    assert len(asyncio.run(scenario())) == 1


def test_cli_lists_and_reports_errors(server: str, capsys: pytest.CaptureFixture[str]) -> None:
    os.environ["LEDGER_URL"] = server
    assert run(["list"]) == 0
    out = capsys.readouterr().out
    assert "USD 1500.00" in out and "sent" in out
    assert run(["void", "4"]) == 1
    assert "invoice 4 is locked because it is paid" in capsys.readouterr().err
    assert run(["create", "Gadgets", "2"]) == 0
    assert "USD 20.00" in capsys.readouterr().out
