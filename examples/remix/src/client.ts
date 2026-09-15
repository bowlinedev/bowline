import { bowlineQuery } from "@bowline/react-query";
import { createClient } from "./bowline.js";

export const client = createClient({ url: "/api" });
export const bq = bowlineQuery(client);
