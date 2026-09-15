import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  timeout: 30_000,
  workers: 1,
  use: { baseURL: "http://localhost:4175" },
  webServer: [
    {
      command: "go run ./cmd/server",
      cwd: "../ledger",
      port: 8080,
      reuseExistingServer: !process.env.CI,
    },
    { command: "pnpm preview", port: 4175, reuseExistingServer: !process.env.CI },
  ],
});
