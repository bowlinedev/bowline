import { BowlineError } from "@bowline/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { bq } from "./api.js";
import type { Invoice } from "./bowline.js";

export function Invoices() {
  const queryClient = useQueryClient();
  const list = useQuery(bq.invoices.list.queryOptions({ limit: 20 }));
  const invalidate = () => queryClient.invalidateQueries({ queryKey: bq.invoices.list.queryKey() });
  const create = useMutation({ ...bq.invoices.create.mutationOptions(), onSuccess: invalidate });
  const voidInvoice = useMutation({ ...bq.invoices.void.mutationOptions(), onSuccess: invalidate });
  const [description, setDescription] = useState("");
  const [quantity, setQuantity] = useState(1);

  function submit(event: FormEvent) {
    event.preventDefault();
    create.mutate({ customerId: 1, lines: [{ description, quantity, unitPrice: "USD 10.00" }] });
  }

  if (list.isPending) {
    return <p>loading</p>;
  }
  if (list.isError) {
    return <p role="alert">{String(list.error)}</p>;
  }
  return (
    <section>
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
                <button
                  type="button"
                  onClick={() => voidInvoice.mutate({ id: invoice.id })}
                  disabled={invoice.status === "void" || invoice.status === "paid"}
                >
                  void
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
