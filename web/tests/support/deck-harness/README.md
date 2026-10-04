# Deck harness

A second entry beside the app, for composites that have no route yet. It renders
one case at a time under `DeckScale`, so a composite can be measured against the
design deck in a browser before a screen uses it. The production build never
includes it, and it has no package script and no committed spec.

## Adding a case

A task adds one file, `cases/<task>.tsx` (for example `cases/r4-02a.tsx`), and
edits nothing else here. The file exports `cases`, a record from a case name to
a component that takes no props:

```tsx
export const cases: Record<string, () => ReactElement> = {
  "row-menu": () => <RowMenu>…</RowMenu>,
};
```

It imports from `@/` and nothing from the harness. The harness gives a case
`DeckScale`, the app's stylesheet and its translations, and nothing else: a case
that needs a router, a query client, a signed-in user or a registered content
element sets that up itself. Write a case that calls a hook as a named function
expression (`"tabs": function DetailTabs() { … }`), which keeps the hook and the
fast-refresh lint rules quiet.

## Opening a case

`index.html?case=<file>/<name>` renders one case, for example
`?case=r4-02a/row-menu`. `&width=<px>` puts it in a container of that width.
With no `case` the page lists every case as a link. Theme and language are the
app's own `localStorage` keys, `quizzivy.theme` and `quizzivy.locale`.

## Building and running

From `web/`:

```sh
./node_modules/.bin/vite build --config tests/support/deck-harness/vite.config.ts
./node_modules/.bin/playwright test --config tests/support/deck-harness/playwright.config.ts
```

The second command serves the build on port 4175 and runs the throwaway specs
named `tests/e2e/zz-harness-*.spec.ts`; none is ever committed. To look at a
case by hand, serve the build with
`pnpm exec vite preview --config tests/support/deck-harness/vite.config.ts` and
open `http://localhost:4175/tests/support/deck-harness/index.html`. Delete
`dist/deck-harness` afterwards.
