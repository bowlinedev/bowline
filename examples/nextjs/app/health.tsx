"use client";

import { useQuery } from "@tanstack/react-query";
import { bq } from "../src/client";

export function Health() {
  const health = useQuery(bq.health.queryOptions());
  return (
    <p data-testid="health">{health.data ? `bowline ${health.data.version}` : "connecting"}</p>
  );
}
