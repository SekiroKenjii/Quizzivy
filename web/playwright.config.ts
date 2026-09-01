import { defineConfig, devices } from "@playwright/test";

/**
 * E2E runs against a real production build served by `vite preview`, not the
 * dev server. §16's exit criteria are about what ships: dev-only behaviour
 * (unminified React warnings, no code splitting applied the same way) would
 * make these tests agree with something users never receive.
 *
 * The full scenario list is spec §14. They arrive with the features they
 * cover — E2E 2a/3/4 in Phase 1, 1a in Phase 2, 5–8 in Phase 3, 1/2/9 in
 * Phase 4. This file and the smoke test exist so that when the first real one
 * is written there is nothing to set up.
 */
export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: true,
  forbidOnly: !!process.env["CI"],
  retries: process.env["CI"] ? 2 : 0,
  workers: process.env["CI"] ? 1 : undefined,
  reporter: process.env["CI"] ? [["github"], ["html", { open: "never" }]] : [["list"]],

  use: {
    baseURL: "http://localhost:4173",
    trace: "on-first-retry",
    // §1.1: the product language. A locale mismatch would make assertions on
    // Vietnamese copy pass or fail for the wrong reason.
    locale: "vi-VN",
    timezoneId: "Asia/Ho_Chi_Minh",
  },

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
      testIgnore: /\.live\.spec\.ts$/,
    },
    /**
     * The one suite that talks to a real API.
     *
     * E2E 1a uploads an actual mp3 so the §11.1 probe runs in the loop: a
     * mocked upload would assert that the frontend can display whatever the
     * mock returns, which is not the thing the phase-2 exit criterion is about.
     * Everything else stays stubbed -- see tests/e2e/support/api.ts for why.
     *
     * Run with `pnpm e2e:live`, which needs postgres, MinIO and the Go API up.
     */
    {
      name: "live",
      testMatch: /\.live\.spec\.ts$/,
      use: { ...devices["Desktop Chrome"] },
    },
    // §16 requires 360px mobile QA and §11.3 calls out iOS Safari specifically.
    // Enabled in T-5.6; declared here so the shape is already right.
    // { name: "mobile-safari", use: { ...devices["iPhone 13"] } },
  ],

  webServer: {
    command: "pnpm build && pnpm preview --port 4173 --strictPort",
    url: "http://localhost:4173",
    reuseExistingServer: !process.env["CI"],
    timeout: 120_000,
    env: {
      // `pnpm build` runs in production mode, so it picks up .env.production --
      // which points at https://api.quizzivy.com. Without this override the
      // E2E suite builds an app that calls the LIVE API: every request from a
      // test run leaves the machine, the route stubs (which match
      // localhost:8080) never fire, and the failure surfaces as a CORS error
      // rather than as "these tests are talking to production".
      //
      // Vite inlines VITE_ variables from process.env as well as from .env
      // files, and process.env wins.
      VITE_API_BASE_URL: "http://localhost:8080",
    },
  },
});
