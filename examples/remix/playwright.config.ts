import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  timeout: 60_000,
  workers: 1,
  use: {
    baseURL: "http://localhost:3200",
    userAgent:
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
  },
  webServer: [
    {
      command: "go run ./cmd/server",
      cwd: "../ledger",
      port: 8080,
      reuseExistingServer: !process.env.CI,
    },
    {
      command: "pnpm start",
      port: 3200,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      env: { PORT: "3200" },
    },
  ],
});
