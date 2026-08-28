// defineConfig comes from vitest/config, not vite -- vite's own type does
// not know about the `test` key.
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
      "@tests": path.resolve(import.meta.dirname, "./tests"),
    },
  },
  server: {
    port: 5173,
    // Both dev servers stay on `localhost`. A 127.0.0.1/localhost split is
    // cross-site and would hide the refresh-cookie behaviour production
    // depends on (docs/plan/00-overview.md §4.1).
    host: "localhost",
  },
  test: {
    // tests/ sits beside src/ and is split by cost:
    //   units/       fast and deterministic -- no build, no browser
    //   integration/ several real layers at once (a real Vite build, MSW +
    //                Testing Library + the real API client)
    //   e2e/         Playwright, run separately by `pnpm e2e`
    //   support/     harness, not tests
    //
    // e2e is excluded explicitly: Vitest's default include matches *.spec.ts,
    // and picking up Playwright specs fails with a confusing "two different
    // versions of @playwright/test".
    include: ["tests/{units,integration}/**/*.test.{ts,tsx}"],
    exclude: ["node_modules/**", "dist/**", "tests/e2e/**"],
    environment: "jsdom",
    globals: true,
    setupFiles: ["./tests/support/setup.ts"],
    css: true,
  },
});
