import { useCallback, useState } from "react";

/**
 * useOverflowsX reports whether the element given to its ref callback is wider
 * inside than it shows, so a horizontal scroll region can take a Tab stop only
 * while there is something to scroll. It measures on mount and again whenever
 * the element or its first child resizes; without ResizeObserver it measures
 * once.
 */
export function useOverflowsX<T extends HTMLElement>(): readonly [
  (node: T | null) => (() => void) | undefined,
  boolean,
] {
  const [overflows, setOverflows] = useState(false);
  const ref = useCallback((node: T | null) => {
    if (node === null) return undefined;
    const measure = () => setOverflows(node.scrollWidth > node.clientWidth);
    measure();
    if (typeof ResizeObserver === "undefined") return undefined;
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    if (node.firstElementChild !== null) observer.observe(node.firstElementChild);
    return () => observer.disconnect();
  }, []);
  return [ref, overflows] as const;
}
