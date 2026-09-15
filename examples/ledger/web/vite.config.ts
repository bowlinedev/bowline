import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: { "/api": "http://localhost:8080", "/ws": { target: "ws://localhost:8080", ws: true } },
  },
  preview: {
    proxy: { "/api": "http://localhost:8080", "/ws": { target: "ws://localhost:8080", ws: true } },
  },
});
