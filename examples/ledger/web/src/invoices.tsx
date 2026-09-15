import { BowlineError } from "@bowline/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useEffect, useState } from "react";
import { bq, client, transportName } from "./api.js";
import type { Attachment, Invoice, ProcedureError } from "./bowline.js";

export function Invoices() {
  const queryClient = useQueryClient();
  const list = useQuery(bq.invoices.list.queryOptions({ limit: 20 }));
  const invalidate = () => queryClient.invalidateQueries({ queryKey: bq.invoices.list.queryKey() });
  const create = useMutation({ ...bq.invoices.create.mutationOptions(), onSuccess: invalidate });
  const [description, setDescription] = useState("");
  const [quantity, setQuantity] = useState(1);
  const [changes, setChanges] = useState(0);
  const [live, setLive] = useState(false);
  const [voidError, setVoidError] = useState<ProcedureError<"invoices.void"> | undefined>();
  const [attached, setAttached] = useState<Attachment | undefined>();

  useEffect(() => {
    const stop = client.invoices.watch.subscribe(
      {},
      {
        onOpen: () => setLive(true),
        onData: () => {
          setChanges((n) => n + 1);
          queryClient.invalidateQueries({ queryKey: bq.invoices.list.queryKey() });
        },
      },
    );
    return stop;
  }, [queryClient]);

  function submit(event: FormEvent) {
    event.preventDefault();
    create.mutate({ customerId: 1, lines: [{ description, quantity, unitPrice: "USD 10.00" }] });
  }

  async function voidInvoice(id: number) {
    setVoidError(undefined);
    const result = await client.invoices.void.safe({ id });
    if (result.ok) {
      invalidate();
    } else {
      setVoidError(result.error);
    }
  }

  async function attach(id: number, file: File | undefined) {
    if (!file) {
      return;
    }
    setAttached(await client.invoices.attach({ invoiceId: id }, file));
  }

  if (list.isPending) {
    return <p>loading</p>;
  }
  if (list.isError) {
    return <p role="alert">{String(list.error)}</p>;
  }
  return (
    <section>
      <p data-testid="transport">
        {transportName} transport, {live ? "live" : "connecting"}, {changes} changes seen
      </p>
      <form onSubmit={submit} style={{ display: "flex", gap: "0.5rem", marginBottom: "1rem" }}>
        <input
          aria-label="description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <input
          aria-label="quantity"
          type="number"
          value={quantity}
          onChange={(e) => setQuantity(Number(e.target.value))}
        />
        <button type="submit" disabled={create.isPending}>
          create invoice
        </button>
      </form>
      {create.error instanceof BowlineError && (
        <ul role="alert" data-testid="issues">
          {create.error.issues.map((issue) => (
            <li key={issue.path.join(".")}>
              {issue.path.join(".")}: {issue.message}
            </li>
          ))}
        </ul>
      )}
      {voidError && (
        <p role="alert" data-testid="void-error">
          {voidError.type === "InvoiceLocked"
            ? `invoice ${voidError.details.id} is locked because it is ${voidError.details.status}`
            : voidError.message}
        </p>
      )}
      {attached && (
        <p data-testid="attached">
          attached {attached.name} ({attached.size} bytes) to invoice {attached.invoiceId}
        </p>
      )}
      <table>
        <thead>
          <tr>
            <th>id</th>
            <th>status</th>
            <th>total</th>
            <th>created</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {list.data.items.map((invoice: Invoice) => (
            <tr key={invoice.id} data-testid={`invoice-${invoice.id}`}>
              <td>{invoice.id}</td>
              <td data-testid="status">{invoice.status}</td>
              <td>{invoice.total}</td>
              <td>{invoice.createdAt.toLocaleDateString()}</td>
              <td>
                <button type="button" onClick={() => voidInvoice(invoice.id)}>
                  void
                </button>
                <input
                  aria-label={`attach to ${invoice.id}`}
                  type="file"
                  onChange={(e) => attach(invoice.id, e.target.files?.[0])}
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
