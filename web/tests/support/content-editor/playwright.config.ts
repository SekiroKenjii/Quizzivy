import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "../../e2e",
  testMatch: "content-editor.spike.ts",
  workers: 1,
  retries: 0,
  reporter: "list",
  use: {
    ...devices["Desktop Chrome"],
    baseURL: "http://localhost:4174",
    trace: "retain-on-failure",
  },
  webServer: {
    command:
      "pnpm exec vite preview --config tests/support/content-editor/vite.config.ts",
    cwd: new URL("../../../", import.meta.url).pathname,
    url: "http://localhost:4174/tests/support/content-editor/index.html",
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
