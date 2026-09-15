<script lang="ts">
import { stores } from "$lib/api.js";
import type { Invoice } from "$lib/bowline.js";
import type { PageData } from "./$types.js";

let { data }: { data: PageData } = $props();

const list = stores.invoices.list.query({ limit: 20 }, { immediate: false });
const create = stores.invoices.create.mutation();
const pending = create.state;

let description = $state("");
let quantity = $state(1);

const invoices = $derived(($list.data?.items ?? data.invoices) as Invoice[]);

async function submit(event: SubmitEvent) {
  event.preventDefault();
  try {
    await create.mutate({
      customerId: 1,
      lines: [{ description, quantity, unitPrice: "USD 10.00" }],
    });
    await list.refresh();
  } catch {
    return;
  }
}
</script>

<h1>Ledger on SvelteKit</h1>
<p data-testid="health">bowline {data.version}</p>

<form onsubmit={submit}>
  <label>
    description
    <input name="description" bind:value={description} />
  </label>
  <label>
    quantity
    <input name="quantity" type="number" bind:value={quantity} />
  </label>
  <button type="submit" disabled={$pending.pending}>create invoice</button>
</form>

{#if $pending.error}
  <ul role="alert" data-testid="issues">
    {#each $pending.error.issues as issue (issue.path.join("."))}
      <li>{issue.path.join(".")}: {issue.message}</li>
    {/each}
    {#if $pending.error.issues.length === 0}
      <li>{$pending.error.message}</li>
    {/if}
  </ul>
{/if}

<table>
  <thead>
    <tr><th>id</th><th>status</th><th>total</th><th>created</th></tr>
  </thead>
  <tbody>
    {#each invoices as invoice (invoice.id)}
      <tr data-testid={`invoice-${invoice.id}`}>
        <td>{invoice.id}</td>
        <td data-testid="status">{invoice.status}</td>
        <td>{invoice.total}</td>
        <td>{new Date(invoice.createdAt).toISOString().slice(0, 10)}</td>
      </tr>
    {/each}
  </tbody>
</table>
