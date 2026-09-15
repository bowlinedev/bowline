import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  timeout: 60_000,
  workers: 1,
  use: { baseURL: "http://localhost:3100" },
  webServer: [
    {
      command: "go run ./cmd/server",
      cwd: "../ledger",
      port: 8080,
      reuseExistingServer: !process.env.CI,
    },
    { command: "pnpm start", port: 3100, reuseExistingServer: !process.env.CI, timeout: 120_000 },
  ],
});
