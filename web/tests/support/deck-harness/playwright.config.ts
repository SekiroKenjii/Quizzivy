import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "../../e2e",
  testMatch: /zz-harness-.*\.spec\.ts$/,
  workers: 1,
  retries: 0,
  reporter: "list",
  use: {
    ...devices["Desktop Chrome"],
    baseURL: "http://localhost:4175",
    trace: "retain-on-failure",
  },
  webServer: {
    command:
      "pnpm exec vite preview --config tests/support/deck-harness/vite.config.ts",
    cwd: new URL("../../../", import.meta.url).pathname,
    url: "http://localhost:4175/tests/support/deck-harness/index.html",
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
