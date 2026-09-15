import { ledger } from "../src/ledger";
import { CreateForm } from "./create-form";
import { Health } from "./health";

export const dynamic = "force-dynamic";

export default async function Page() {
  const api = await ledger({ next: { tags: ["invoices"] } });
  const page = await api.invoices.list({ limit: 20 });
  return (
    <main>
      <h1>Ledger</h1>
      <Health />
      <table>
        <thead>
          <tr>
            <th>id</th>
            <th>status</th>
            <th>total</th>
          </tr>
        </thead>
        <tbody>
          {page.items.map((invoice) => (
            <tr key={invoice.id} data-testid={`invoice-${invoice.id}`}>
              <td>{invoice.id}</td>
              <td data-testid="status">{invoice.status}</td>
              <td>{invoice.total}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p data-testid="count">{page.items.length} invoices rendered on the server</p>
      <CreateForm />
    </main>
  );
}
