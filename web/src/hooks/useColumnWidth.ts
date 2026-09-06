import { useCallback, useState } from "react";

/** F-13: the four side-column roles, each with one remembered width. */
export type ColumnRole = "sidebar" | "rail" | "panel" | "outline";

export interface ColumnLimits {
  /** Pixels; the deck's defaults are F-11's 14rem / 20rem, the shell's 15rem, A-04's 18rem. */
  fallback: number;
  min: number;
  max: number;
}

export const COLUMN_LIMITS: Record<ColumnRole, ColumnLimits> = {
  sidebar: { fallback: 240, min: 192, max: 320 },
  rail: { fallback: 224, min: 192, max: 320 },
  panel: { fallback: 320, min: 256, max: 512 },
  outline: { fallback: 288, min: 224, max: 384 },
};

/** The middle column never drops under this, whatever the sides are dragged to (F-12). */
export const MIDDLE_MIN = 384;

const KEY = (role: ColumnRole) => `quizzivy.column.${role}`;

function read(role: ColumnRole): number | null {
  try {
    const raw = localStorage.getItem(KEY(role));
    const value = raw === null ? Number.NaN : Number(raw);
    return Number.isFinite(value) ? value : null;
  } catch {
    return null;
  }
}

function write(role: ColumnRole, width: number | null) {
  try {
    if (width === null) localStorage.removeItem(KEY(role));
    else localStorage.setItem(KEY(role), String(width));
  } catch {
    // A private window or blocked storage only costs the memory, never the resize.
  }
}

export function clampWidth(width: number, limits: ColumnLimits): number {
  return Math.min(limits.max, Math.max(limits.min, Math.round(width)));
}

/**
 * One width per role, remembered in this browser. Every screen that draws a
 * panel shares the panel's width, which is F-11's "one width each" with the
 * teacher allowed to pick the one.
 */
export function useColumnWidth(role: ColumnRole) {
  const limits = COLUMN_LIMITS[role];
  const [width, setState] = useState(() => {
    const stored = read(role);
    return stored === null ? limits.fallback : clampWidth(stored, limits);
  });
  const setWidth = useCallback(
    (next: number) => {
      const clamped = clampWidth(next, limits);
      setState(clamped);
      write(role, clamped);
    },
    [role, limits],
  );
  const reset = useCallback(() => {
    setState(limits.fallback);
    write(role, null);
  }, [role, limits]);
  return { width, setWidth, reset, limits };
}
