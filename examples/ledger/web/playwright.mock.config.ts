import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  testMatch: "mock.spec.ts",
  timeout: 30_000,
  workers: 1,
  use: { baseURL: "http://localhost:4173" },
  webServer: [
    {
      command: "bash ./e2e/mock.sh",
      url: "http://localhost:8080/api/health",
      reuseExistingServer: false,
      timeout: 180_000,
    },
    { command: "pnpm preview", port: 4173, reuseExistingServer: !process.env.CI },
  ],
});
