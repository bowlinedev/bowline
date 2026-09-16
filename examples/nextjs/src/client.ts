import { bowlineQuery } from "@bowlinedev/react-query";
import { createClient } from "./bowline";

export const client = createClient({ url: "/api" });
export const bq = bowlineQuery(client);
