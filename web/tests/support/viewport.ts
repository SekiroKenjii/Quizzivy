import { vi } from "vitest";

/**
 * The student shell and its detail pages branch on 768px in code, and the
 * engine on 1024px, so a test says which side it is on. jsdom's default stub answers
 * "wide" to every min-width query; a test of a phone board pins "phone". The
 * returned `resize` moves to the other side and tells every listener, for a
 * test of what survives the crossing.
 */
export function viewport(width: "phone" | "desktop") {
  let wide = width === "desktop";
  const listeners = new Set<() => void>();
  vi.stubGlobal(
    "matchMedia",
    (query: string) =>
      ({
        get matches() {
          return wide && query.includes("min-width");
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
    resize(next: "phone" | "desktop") {
      wide = next === "desktop";
      for (const listener of [...listeners]) listener();
    },
  };
}
