# Tests

`tests/` sits beside `src/` so the source tree contains source and nothing else.
The split is by **cost**, which is what makes it useful — you can run the cheap
ones on every save and the expensive ones on demand.

| Directory      | What belongs here                                                                                                           | Cost        |
| -------------- | --------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `units/`       | Fast, deterministic, no build and no browser. Stub the boundary.                                                            | ~ms         |
| `integration/` | Several real layers at once — a real Vite build, or Testing Library + MSW + the real API client + TanStack Query + i18next. | ~100s of ms |
| `e2e/`         | Playwright, against a real production build.                                                                                | seconds     |
| `support/`     | Harness, not tests. MSW server and handlers, fixtures, the ajv contract validator, the OpenAPI reader.                      | —           |

```bash
pnpm test              # units + integration
pnpm test:unit         # just units
pnpm test:integration  # just integration
pnpm e2e               # Playwright
```

`e2e/` is excluded from Vitest explicitly. Vitest's default include matches
`*.spec.ts`, and picking up Playwright specs fails with a confusing "two
different versions of @playwright/test".

## Local runs

Outside CI the suite runs with three workers and a 15-second test limit
(`vite.config.ts`). CI keeps vitest's defaults, so a test that is too slow still
fails there. Pass `--maxWorkers=<n>` to use more workers on an idle machine.

## Where a new test goes

Ask what it would take to make it fail for a reason unrelated to the thing it
tests. If the answer is "a slow disk" or "a flaky browser", it is not a unit
test.

- Stubs `fetch`, reads a file, parses the contract → **units**
- Renders a component that fetches → **integration**
- Drives a browser → **e2e**

## Imports

`@/…` is `src/`, `@tests/…` is `tests/`. Tests import production code through
`@/` like anything else; there are no deep relative paths between the two trees.

## Two harness pieces worth knowing about

**`support/contractResponse.ts`** validates every MSW fixture against
`api/openapi.yaml` with ajv before returning it. A hand-written mock is a
second, unversioned implementation of the API: without this, someone adds a
field to the contract, the handler keeps returning the old shape, and component
tests stay green about a payload production will never send. A drifted mock
fails at the mock rather than inside whatever it was meant to be testing.

**`units/contract/`** asserts structural invariants of the contract itself —
the §13.5 student-payload boundary, the §6.5 public-endpoint rules, pagination,
`operationId` uniqueness. Its helpers are unit-tested against synthetic
documents, because a checker nobody checks is decoration.

**`units/styles/tailwind-classes.test.ts`** reads every colour utility (`bg-`,
`text-`, `border-` and its side forms, `ring-`, `shadow-` and the rest) in `src/`
and fails when one names no `--color-*` or `--shadow-*` key of `index.css`'s
`@theme`, no Tailwind default and no non-colour utility. Tailwind generates no
CSS for such a class, and a test that only asserts the class name passes anyway;
`bg-accent-soft` shipped that way.

The scan is a token regex over each file's text with its comments removed, not a
parse, so it sees a class the same way inside a template literal, `cn(...)`,
`cva(...)` and `clsx(...)`, and it cannot tell a class from a string that looks
like one. Variants, the `/50` opacity suffix and the `!` marker are stripped
first. A value in brackets or parentheses (`text-[13px]`, `bg-(--x)`) is
arbitrary and skipped. The longest prefix wins, so `border-t-brand` is read as
side `border-t` with colour `brand`, not as `border` with `t-brand`. A new
non-colour suffix (say `border-spacing-*`) goes in the test's `NON_COLOUR` table,
and a CSS word that merely starts with a prefix (`border-box`) in `CSS_WORDS`.
