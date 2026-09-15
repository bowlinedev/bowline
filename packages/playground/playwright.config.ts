import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  timeout: 60_000,
  workers: 1,
  use: { baseURL: "http://127.0.0.1:18090" },
  webServer: {
    command: "bash ./e2e/serve.sh",
    url: "http://127.0.0.1:18090/_playground/contract.json",
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
  },
});
