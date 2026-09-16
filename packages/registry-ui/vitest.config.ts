import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  test: {
    include: ["src/**/*.test.ts"],
  },
  resolve: {
    alias: {
      "@bowlinedev/client": fileURLToPath(new URL("../client/src/index.ts", import.meta.url)),
    },
  },
});
