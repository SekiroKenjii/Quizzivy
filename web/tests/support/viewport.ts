import { vi } from "vitest";

/**
 * The student shell and the engine branch on 1024px in code, not only in
 * CSS, so a test says which side it is on. jsdom's default stub answers
 * "wide" to every min-width query; a test of a phone board pins "phone".
 */
export function viewport(width: "phone" | "desktop") {
  const wide = width === "desktop";
  vi.stubGlobal(
    "matchMedia",
    (query: string) =>
      ({
        matches: wide && query.includes("min-width"),
        media: query,
        onchange: null,
        addEventListener: () => {},
        removeEventListener: () => {},
        addListener: () => {},
        removeListener: () => {},
        dispatchEvent: () => false,
      }) as MediaQueryList,
  );
}
