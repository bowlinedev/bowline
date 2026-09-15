import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig, type PluginOption } from "vite";

export default defineConfig({
  plugins: [sveltekit() as unknown as PluginOption],
  server: { proxy: { "/api": "http://localhost:8080" } },
  preview: { proxy: { "/api": "http://localhost:8080" } },
});
