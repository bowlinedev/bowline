import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  timeout: 60_000,
  workers: 1,
  use: { baseURL: "http://localhost:4176" },
  webServer: [
    {
      command: "bash ./e2e/services.sh",
      url: "http://127.0.0.1:8090/api/.bowline/health",
      reuseExistingServer: false,
      timeout: 180_000,
    },
    { command: "pnpm preview", port: 4176, reuseExistingServer: !process.env.CI },
  ],
});
