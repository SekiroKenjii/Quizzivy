// defineConfig comes from vitest/config, not vite -- vite's own type does
// not know about the `test` key.
import { defineConfig } from "vitest/config";
import type { Plugin } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { readFileSync } from "node:fs";
import path from "node:path";

/**
 * appVersion compiles the build id and web/package.json's version in as
 * __APP_BUILD__ and __APP_VERSION__, writes the version into index.html's
 * static splash, and publishes the pair as /version.json -- in the build output
 * and from the dev server -- so an open tab can tell a newer deploy from its
 * own. The deploy sets QUIZZIVY_BUILD to the commit it ships; any other build
 * gets an id of its own.
 */
function appVersion(): Plugin {
  const pkg = JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, "package.json"), "utf8"),
  ) as { version: string };
  const build = process.env["QUIZZIVY_BUILD"] || `local-${Date.now().toString(36)}`;
  const body = JSON.stringify({ build, version: pkg.version });
  return {
    name: "quizzivy:app-version",
    config: () => ({
      define: {
        __APP_BUILD__: JSON.stringify(build),
        __APP_VERSION__: JSON.stringify(pkg.version),
      },
    }),
    transformIndexHtml: (html) => html.replaceAll("%QUIZZIVY_VERSION%", pkg.version),
    configureServer(server) {
      server.middlewares.use("/version.json", (_request, response) => {
        response.setHeader("Content-Type", "application/json");
        response.setHeader("Cache-Control", "no-store");
        response.end(body);
      });
    },
    generateBundle() {
      this.emitFile({ type: "asset", fileName: "version.json", source: body });
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), appVersion()],
  // One .env for the whole repository, at the root -- which is where
  // .env.example lives and where it documents BOTH halves of the config, the
  // server's and the VITE_ ones. Vite otherwise looks only in web/, so every
  // VITE_ value in that file was silently unset and the frontend ran on its
  // fallbacks: VITE_GOOGLE_CLIENT_ID missing simply hid the Google button.
  envDir: path.resolve(import.meta.dirname, ".."),
  // Explicit, because of what it is holding back. That root .env is expected to
  // hold POSTGRES_SUPERUSER_PASSWORD, JWT_SIGNING_KEY, GOOGLE_CLIENT_SECRET and
  // both sets of object-storage credentials -- so the ONLY thing keeping them
  // out of a bundle any visitor can read is which prefix gets inlined. Leaving
  // it to a framework default put a lot of weight on something invisible.
  //
  // This does not protect against the other half: a variable NAMED with a VITE_
  // prefix by mistake is inlined however explicit this is, and .env.example
  // lists VITE_GOOGLE_CLIENT_ID directly above GOOGLE_CLIENT_SECRET. That case
  // is caught by tests/integration/bundle-secrets.test.ts.
  envPrefix: ["VITE_"],
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
    // The tests state their own preconditions rather than inheriting them from
    // an untracked file.
    //
    // envDir points at the repository root, where VITE_GOOGLE_CLIENT_ID lives
    // in .env -- which is gitignored. So a developer's machine rendered the
    // Google button and CI did not, and `/login`'s Google test failed on every
    // run for twelve runs while passing for everyone locally. A test that only
    // passes on machines holding a secret is not testing what it claims to.
    //
    // The value is a placeholder: nothing in a unit test reaches Google, and
    // the only thing read from it is whether it is non-empty.
    env: {
      VITE_GOOGLE_CLIENT_ID: "test-client-id.apps.googleusercontent.com",
    },
  },
});
