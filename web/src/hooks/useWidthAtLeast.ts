import { useCallback, useState } from "react";

/**
 * useWidthAtLeast reports whether the element given to its ref callback is at
 * least `px` wide, measured on mount and whenever it resizes. Until the
 * element has a width (no layout yet, jsdom, `display: none`) the answer is
 * true, and without ResizeObserver it is measured once. The consumer
 * re-renders only when the answer changes.
 */
export function useWidthAtLeast<T extends HTMLElement>(
  px: number,
): readonly [(node: T | null) => (() => void) | undefined, boolean] {
  const [atLeast, setAtLeast] = useState(true);
  const ref = useCallback(
    (node: T | null) => {
      if (node === null) return undefined;
      const measure = () => {
        const width = node.getBoundingClientRect().width;
        if (width > 0) setAtLeast(width >= px);
      };
      measure();
      if (typeof ResizeObserver === "undefined") return undefined;
      const observer = new ResizeObserver(measure);
      observer.observe(node);
      return () => observer.disconnect();
    },
    [px],
  );
  return [ref, atLeast] as const;
}
