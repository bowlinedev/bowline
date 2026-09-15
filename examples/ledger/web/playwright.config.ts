import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  timeout: 30_000,
  use: { baseURL: "http://localhost:4173" },
  webServer: [
    { command: "go run ./cmd/server", cwd: "..", port: 8080, reuseExistingServer: !process.env.CI },
    { command: "pnpm preview", port: 4173, reuseExistingServer: !process.env.CI },
  ],
});
