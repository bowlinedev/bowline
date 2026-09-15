import { bowlineSolid } from "@bowline/solid";
import { createSignal, For, Show } from "solid-js";
import { client } from "../api.js";

const ledger = bowlineSolid(client);

export default function Invoices() {
  const [list, { refetch }] = ledger.invoices.list.resource({ limit: 20 });
  const [create, state] = ledger.invoices.create.action();
  const [description, setDescription] = createSignal("");
  const [quantity, setQuantity] = createSignal(1);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    try {
      await create({
        customerId: 1,
        lines: [{ description: description(), quantity: quantity(), unitPrice: "USD 10.00" }],
      });
      setDescription("");
      await refetch();
    } catch (error) {
      if (!state().error) {
        throw error;
      }
    }
  }

  return (
    <div>
      <form onSubmit={submit}>
        <label>
          description
          <input
            aria-label="description"
            value={description()}
            onInput={(event) => setDescription(event.currentTarget.value)}
          />
        </label>
        <label>
          quantity
          <input
            aria-label="quantity"
            type="number"
            value={quantity()}
            onInput={(event) => setQuantity(Number(event.currentTarget.value))}
          />
        </label>
        <button type="submit" disabled={state().pending}>
          create invoice
        </button>
      </form>
      <Show when={state().error}>
        {(error) => (
          <ul role="alert" data-testid="issues">
            <For each={error().issues}>
              {(issue) => (
                <li>
                  {issue.path.join(".")}: {issue.message}
                </li>
              )}
            </For>
          </ul>
        )}
      </Show>
      <table>
        <tbody>
          <For each={list()?.items ?? []}>
            {(invoice) => (
              <tr data-testid={`island-invoice-${invoice.id}`}>
                <td>{invoice.id}</td>
                <td>{invoice.status}</td>
                <td>{invoice.total}</td>
              </tr>
            )}
          </For>
        </tbody>
      </table>
    </div>
  );
}
