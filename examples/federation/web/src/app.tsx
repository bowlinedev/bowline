import { BowlineError } from "@bowline/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { bq } from "./api.js";

export function App() {
  const queryClient = useQueryClient();
  const [error, setError] = useState<BowlineError | undefined>();
  const invoices = useQuery(bq.ledger.invoices.list.queryOptions({ limit: 20 }));
  const charges = useQuery(bq.billing.charges.list.queryOptions({ limit: 20 }));
  const createInvoice = useMutation({
    ...bq.ledger.invoices.create.mutationOptions(),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: bq.ledger.invoices.list.queryKey() }),
  });
  const createCharge = useMutation({
    ...bq.billing.charges.create.mutationOptions(),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: bq.billing.charges.list.queryKey() }),
  });

  async function chargeInvoice(invoiceId: number) {
    setError(undefined);
    try {
      await createCharge.mutateAsync({ invoiceId });
    } catch (thrown) {
      if (thrown instanceof BowlineError) {
        setError(thrown);
        return;
      }
      throw thrown;
    }
  }

  return (
    <main>
      <h1>Federation</h1>
      <section>
        <h2>Invoices</h2>
        <button
          type="button"
          onClick={() =>
            createInvoice.mutate({
              customerId: 1,
              lines: [{ description: "Consulting", quantity: 2, unitPrice: "USD 10.00" }],
            })
          }
        >
          create invoice
        </button>
        <ul>
          {invoices.data?.items.map((invoice) => (
            <li key={invoice.id} data-testid={`invoice-${invoice.id}`}>
              {invoice.total} {invoice.status}
              <button type="button" onClick={() => chargeInvoice(invoice.id)}>
                charge {invoice.id}
              </button>
            </li>
          ))}
        </ul>
      </section>
      <section>
        <h2>Charges</h2>
        {error && (
          <p role="alert" data-testid="charge-error">
            {error.code}: {error.message}
          </p>
        )}
        <ul>
          {charges.data?.items.map((charge) => (
            <li key={charge.id} data-testid={`charge-${charge.id}`}>
              invoice {charge.invoiceId} {charge.amount} {charge.status}
            </li>
          ))}
        </ul>
      </section>
    </main>
  );
}
