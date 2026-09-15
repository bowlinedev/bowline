from __future__ import annotations

import argparse
import os
import sys

from bowline_client import BowlineError, SyncTransport

from bowline import (
    CreateInvoiceInput,
    GetInvoiceInput,
    Invoice,
    InvoiceLocked,
    Line,
    ListInvoicesInput,
    SyncClient,
    VoidInvoiceInput,
)


def connect() -> SyncClient:
    url = os.environ.get("LEDGER_URL", "http://localhost:8080/api")
    token = os.environ.get("LEDGER_TOKEN")
    headers = (lambda: {"authorization": f"Bearer {token}"}) if token else None
    return SyncClient(SyncTransport(url, headers=headers))


def show(invoice: Invoice) -> None:
    created = invoice.createdAt.astimezone().strftime("%Y-%m-%d %H:%M")
    print(f"{invoice.id:>4}  {invoice.status.value:<6} {invoice.total:>14}  {created}")


def run(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(prog="ledger", description="Talk to the ledger example.")
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("list", help="list invoices")
    create = commands.add_parser("create", help="create an invoice for customer 1")
    create.add_argument("description")
    create.add_argument("quantity", type=int)
    void = commands.add_parser("void", help="void an invoice")
    void.add_argument("id", type=int)
    get = commands.add_parser("get", help="show one invoice")
    get.add_argument("id", type=int)
    args = parser.parse_args(argv)
    client = connect()
    try:
        if args.command == "list":
            page = client.invoices.list(ListInvoicesInput(limit=50))
            for invoice in page.items:
                show(invoice)
        elif args.command == "create":
            line = Line(description=args.description, quantity=args.quantity, unitPrice="USD 10.00")
            show(client.invoices.create(CreateInvoiceInput(customerId=1, lines=[line])))
        elif args.command == "void":
            show(client.invoices.void(VoidInvoiceInput(id=args.id)))
        elif args.command == "get":
            show(client.invoices.get(GetInvoiceInput(id=args.id)))
    except BowlineError as err:
        if err.type == "InvoiceLocked":
            locked = err.details_as(InvoiceLocked)
            print(
                f"invoice {locked.id} is locked because it is {locked.status.value}",
                file=sys.stderr,
            )
        else:
            print(f"{err.code.value}: {err.message}", file=sys.stderr)
            for issue in err.issues:
                print(f"  {'.'.join(issue.path)}: {issue.message}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(run(sys.argv[1:]))
