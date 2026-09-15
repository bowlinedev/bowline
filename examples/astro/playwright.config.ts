import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  timeout: 30_000,
  workers: 1,
  use: { baseURL: "http://localhost:4321" },
  webServer: [
    {
      command: "go run ./cmd/server",
      cwd: "../ledger",
      port: 8080,
      reuseExistingServer: !process.env.CI,
    },
    {
      command: "pnpm start",
      port: 4321,
      reuseExistingServer: !process.env.CI,
      env: { HOST: "127.0.0.1", PORT: "4321" },
    },
  ],
});
