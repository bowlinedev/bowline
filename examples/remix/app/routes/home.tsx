import { BowlineError } from "@bowline/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { useRevalidator } from "react-router";
import { bq } from "../../src/client.js";
import { ledger } from "../../src/ledger.js";
import type { Route } from "./+types/home";

export async function loader({ request }: Route.LoaderArgs) {
  const page = await ledger(request).invoices.list({ limit: 20 });
  return { invoices: page.items };
}

export default function Home({ loaderData }: Route.ComponentProps) {
  const revalidator = useRevalidator();
  const queryClient = useQueryClient();
  const health = useQuery(bq.health.queryOptions());
  const create = useMutation({
    ...bq.invoices.create.mutationOptions(),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: bq.invoices.list.queryKey() });
      await revalidator.revalidate();
    },
  });
  const [description, setDescription] = useState("");
  const [quantity, setQuantity] = useState(1);

  function submit(event: FormEvent) {
    event.preventDefault();
    create.mutate({ customerId: 1, lines: [{ description, quantity, unitPrice: "USD 10.00" }] });
  }

  return (
    <main>
      <h1>Ledger</h1>
      <p data-testid="health">{health.data ? `bowline ${health.data.version}` : "connecting"}</p>
      <table>
        <thead>
          <tr>
            <th>id</th>
            <th>status</th>
            <th>total</th>
          </tr>
        </thead>
        <tbody>
          {loaderData.invoices.map((invoice) => (
            <tr key={invoice.id} data-testid={`invoice-${invoice.id}`}>
              <td>{invoice.id}</td>
              <td data-testid="status">{invoice.status}</td>
              <td>{invoice.total}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p data-testid="count">{loaderData.invoices.length} invoices rendered on the server</p>
      <form onSubmit={submit}>
        <label>
          description
          <input
            aria-label="description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
        <label>
          quantity
          <input
            aria-label="quantity"
            type="number"
            value={quantity}
            onChange={(e) => setQuantity(Number(e.target.value))}
          />
        </label>
        <button type="submit" disabled={create.isPending}>
          create invoice
        </button>
        {create.error instanceof BowlineError && (
          <ul role="alert" data-testid="issues">
            {create.error.issues.map((issue) => (
              <li key={issue.path.join(".")}>
                {issue.path.join(".")}: {issue.message}
              </li>
            ))}
          </ul>
        )}
        {create.data && <p data-testid="created">created invoice {create.data.id}</p>}
      </form>
    </main>
  );
}
