import node from "@astrojs/node";
import solid from "@astrojs/solid-js";
import { defineConfig } from "astro/config";

const proxy = { "/api": "http://localhost:8080" };

export default defineConfig({
  output: "server",
  adapter: node({ mode: "standalone" }),
  integrations: [solid()],
  vite: {
    server: { proxy },
    preview: { proxy },
  },
});
