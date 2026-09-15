"use client";

import { useMutation } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { type CreateResult, createInvoice } from "./actions";

export function CreateForm() {
  const [description, setDescription] = useState("");
  const [quantity, setQuantity] = useState(1);
  const create = useMutation<CreateResult, Error, { description: string; quantity: number }>({
    mutationFn: (input) => createInvoice(input.description, input.quantity),
  });

  function submit(event: FormEvent) {
    event.preventDefault();
    create.mutate({ description, quantity });
  }

  const outcome = create.data;
  return (
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
      {outcome && !outcome.ok && (
        <ul role="alert" data-testid="issues">
          {outcome.issues.map((issue) => (
            <li key={issue.path.join(".")}>
              {issue.path.join(".")}: {issue.message}
            </li>
          ))}
        </ul>
      )}
      {outcome?.ok && <p data-testid="created">created invoice {outcome.id}</p>}
    </form>
  );
}
