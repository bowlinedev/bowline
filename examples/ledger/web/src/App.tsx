import { useQuery } from "@tanstack/react-query";
import { bq } from "./api.js";
import { Invoices } from "./invoices.js";

export function App() {
  const health = useQuery(bq.health.queryOptions());
  return (
    <main
      style={{ fontFamily: "system-ui", maxWidth: 720, margin: "2rem auto", padding: "0 1rem" }}
    >
      <h1>Ledger</h1>
      <p data-testid="health">{health.data ? `bowline ${health.data.version}` : "connecting"}</p>
      <Invoices />
    </main>
  );
}
