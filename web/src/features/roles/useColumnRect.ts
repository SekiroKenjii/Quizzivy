import { useCallback, useState } from "react";

/** ColumnRect is where a role's column sits inside the matrix: its left edge and width in pixels. */
export interface ColumnRect {
  left: number;
  width: number;
}

const BLEED = 4;

/**
 * useColumnRect measures the column of the header marked `data-role-col` with
 * `roleId` inside the element given to its ref callback, and again whenever
 * that element resizes, so the frame follows a sidebar that collapses. The
 * rect is the header's box widened by 4px each side, as the deck frames it,
 * and null while no role is highlighted or its header is absent.
 */
export function useColumnRect(roleId: string | null) {
  const [rect, setRect] = useState<ColumnRect | null>(null);
  const ref = useCallback(
    (root: HTMLElement | null) => {
      if (root === null) return undefined;
      if (roleId === null) {
        setRect(null);
        return undefined;
      }
      const measure = () => {
        const head = Array.from(
          root.querySelectorAll<HTMLElement>("[data-role-col]"),
        ).find((node) => node.dataset["roleCol"] === roleId);
        if (head === undefined) {
          setRect(null);
          return;
        }
        const frame = root.getBoundingClientRect();
        const box = head.getBoundingClientRect();
        const next = {
          left: Math.round(box.left - frame.left - BLEED),
          width: Math.round(box.width + 2 * BLEED),
        };
        setRect((was) =>
          was !== null && was.left === next.left && was.width === next.width
            ? was
            : next,
        );
      };
      measure();
      if (typeof ResizeObserver === "undefined") return undefined;
      const observer = new ResizeObserver(measure);
      observer.observe(root);
      return () => observer.disconnect();
    },
    [roleId],
  );
  return [ref, roleId === null ? null : rect] as const;
}
