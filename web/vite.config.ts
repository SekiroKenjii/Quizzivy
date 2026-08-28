// defineConfig comes from vitest/config, not vite -- vite's own type does
// not know about the `test` key.
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(import.meta.dirname, "./src") },
  },
  server: {
    port: 5173,
    // Both dev servers stay on `localhost`. A 127.0.0.1/localhost split is
    // cross-site and would hide the refresh-cookie behaviour production
    // depends on (docs/plan/00-overview.md §4.1).
    host: "localhost",
  },
  test: {
    // Playwright specs live in e2e/ and are run by `pnpm e2e`. Vitest's default
    // include pattern matches *.spec.ts, so without this it tries to run them
    // and fails with a confusing "two different versions of @playwright/test".
    exclude: ["node_modules/**", "dist/**", "e2e/**"],
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    css: true,
  },
});
