import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import jsxA11y from "eslint-plugin-jsx-a11y";
import react from "eslint-plugin-react";
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
      react,
    },
    rules: {
      ...reactHooks.configs["recommended-latest"].rules,
      ...jsxA11y.flatConfigs.recommended.rules,
      "react-refresh/only-export-components": ["warn", { allowConstantExport: true }],

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

      // §14: "All strings via t(), keys in both vi and en." Vietnamese is the
      // product language and AGENTS.md forbids English-only text reaching a
      // commit -- this is what stops a hardcoded string sneaking in.
      //
      // Scope: JSX text children only (ignoreProps). Checking every attribute
      // would flag className, id and type, which is unusable. Hardcoded
      // user-facing ATTRIBUTES -- aria-label, placeholder, title -- are covered
      // by src/lib/i18n/no-hardcoded-strings.test.ts instead, which can be
      // precise about which attributes matter.
      "react/jsx-no-literals": [
        "error",
        {
          noStrings: true,
          ignoreProps: true,
          allowedStrings: ["·", "—", "–", "/", "|", ":", "×", "…"],
        },
      ],
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
    files: ["**/*.config.{js,ts}", "tests/**"],
    languageOptions: { globals: { ...globals.node, ...globals.browser } },
  },
  {
    // Test fixtures render marker text -- "admin home", "login page" -- to
    // assert which route won. Those are not user-facing strings and have no
    // business in the locale files; putting them through t() would mean the
    // test asserted its own translation rather than the routing.
    //
    // Narrow on purpose: the rule stays on for everything under src/, which is
    // the only place §14's requirement applies.
    files: ["tests/**"],
    rules: { "react/jsx-no-literals": "off" },
  },
];
