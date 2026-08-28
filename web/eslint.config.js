import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import jsxA11y from "eslint-plugin-jsx-a11y";
import tseslint from "typescript-eslint";

// Plugins are registered explicitly rather than through `extends`. The plugins
// disagree about how they export flat configs, and composing their presets
// produced an unhelpful "Flat config requires plugins to be an object" error
// that named the wrong plugin. Listing rules by hand is longer but says exactly
// what is enforced, which matters more here than brevity.

export default [
  {
    ignores: [
      "dist/**",
      "node_modules/**",
      // Generated from api/openapi.yaml by `make gen`. CI fails on drift, so
      // linting it would only ever produce noise nobody may act on.
      "src/lib/api/schema.d.ts",
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
      "jsx-a11y": jsxA11y,
    },
    rules: {
      ...reactHooks.configs["recommended-latest"].rules,
      ...jsxA11y.flatConfigs.recommended.rules,
      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true },
      ],

      // §14: "no `any` without a comment". An explicit `any` should have to be
      // argued for in the diff, so this is an error, not a warning.
      "@typescript-eslint/no-explicit-any": "error",
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],

      // §14 requires keyboard operability throughout; these catch the usual
      // ways it gets lost.
      "jsx-a11y/no-static-element-interactions": "error",
      "jsx-a11y/click-events-have-key-events": "error",
      "jsx-a11y/no-autofocus": "warn",
    },
  },
  {
    // shadcn primitives are vendored by `pnpm dlx shadcn add`, not hand-written.
    // Their canonical shape exports a component alongside its cva variants,
    // which the fast-refresh rule dislikes. Editing them to satisfy it would be
    // undone by the next `shadcn add`.
    files: ["src/components/ui/**"],
    rules: { "react-refresh/only-export-components": "off" },
  },
  {
    files: ["**/*.config.{js,ts}", "src/test/**"],
    languageOptions: { globals: { ...globals.node, ...globals.browser } },
  },
];
