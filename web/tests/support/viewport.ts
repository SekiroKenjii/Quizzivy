import { vi } from "vitest";

type Width = "phone" | "desktop" | number;

const BOUND = /\((min|max)-width:\s*(\d+(?:\.\d+)?)px\)/g;

function answer(width: Width, query: string) {
  if (typeof width !== "number")
    return width === "desktop" && query.includes("min-width");
  const bounds = [...query.matchAll(BOUND)];
  return (
    bounds.length > 0 &&
    bounds.every(([, side, px]) =>
      side === "min" ? width >= Number(px) : width <= Number(px),
    )
  );
}

/**
 * viewport pins the width a test runs at by stubbing `matchMedia`; jsdom's
 * default stub answers "wide" to every min-width query. The two named forms
 * are for a surface with one breakpoint, as the student shell, its detail
 * pages and the engine branch on 768px: `"desktop"` answers true to every
 * query containing `min-width` and `"phone"` false to every query. A number
 * of CSS pixels is for the teacher console, which has states between its
 * thresholds (768, 1024, 1100): `(min-width: Npx)` matches when the width is
 * at least N and `(max-width: Npx)` when it is at most N, a query with both
 * needs both, and any other query answers false. The returned `resize` moves
 * to another width, in either form, and tells every listener, for a test of
 * what survives the crossing.
 */
export function viewport(width: Width) {
  let current = width;
  const listeners = new Set<() => void>();
  vi.stubGlobal(
    "matchMedia",
    (query: string) =>
      ({
        get matches() {
          return answer(current, query);
        },
        media: query,
        onchange: null,
        addEventListener: (_: string, listener: () => void) => listeners.add(listener),
        removeEventListener: (_: string, listener: () => void) =>
          listeners.delete(listener),
        addListener: () => {},
        removeListener: () => {},
        dispatchEvent: () => false,
      }) as unknown as MediaQueryList,
  );
  return {
    resize(next: Width) {
      current = next;
      for (const listener of [...listeners]) listener();
    },
  };
}
