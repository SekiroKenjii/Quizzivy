import { useCallback, useSyncExternalStore } from "react";

/**
 * Subscribes to a media query.
 *
 * `useSyncExternalStore` rather than useState + useEffect: matchMedia is
 * external state, and this is the primitive React provides for it. The effect
 * version has to write state during the effect to resync after mount, which is
 * both an anti-pattern and a real tearing risk during concurrent rendering.
 */
export function useMediaQuery(query: string): boolean {
  const subscribe = useCallback(
    (onChange: () => void) => {
      const mql = window.matchMedia(query);
      mql.addEventListener("change", onChange);
      return () => mql.removeEventListener("change", onChange);
    },
    [query],
  );

  const getSnapshot = useCallback(() => window.matchMedia(query).matches, [query]);

  // Server/prerender snapshot. Nothing prerenders today (SPA mode, §2), but
  // useSyncExternalStore requires it and `false` matches the wider layout.
  const getServerSnapshot = useCallback(() => false, []);

  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
