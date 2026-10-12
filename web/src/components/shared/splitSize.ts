/** SplitUnit identifies pixels or a percentage of the pane row. */
export type SplitUnit = "px" | "percent";

/** clampSplitSize keeps a finite size inside the limits, honoring the minimum when the ceiling is lower. */
export function clampSplitSize(size: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, Number.isFinite(size) ? size : min));
}

/** pixelSplitSize applies a pointer delta to the initial pixel size and clamps it. */
export function pixelSplitSize(
  size: number,
  startX: number,
  clientX: number,
  min: number,
  max: number,
): number {
  return clampSplitSize(Math.round(size + clientX - startX), min, max);
}

/** percentSplitSize converts a pointer position into a clamped percentage of the row. */
export function percentSplitSize(
  clientX: number,
  left: number,
  width: number,
  min: number,
  max: number,
): number {
  return clampSplitSize(width > 0 ? ((clientX - left) / width) * 100 : min, min, max);
}

/** nextSplitSize returns a clamped keyboard size, or null for an unhandled key. */
export function nextSplitSize(
  key: string,
  size: number,
  step: number,
  min: number,
  max: number,
): number | null {
  switch (key) {
    case "ArrowLeft":
      return clampSplitSize(size - step, min, max);
    case "ArrowRight":
      return clampSplitSize(size + step, min, max);
    case "Home":
      return min;
    case "End":
      return clampSplitSize(max, min, max);
    default:
      return null;
  }
}

/** readSplitSize restores a finite remembered size or the clamped default when storage is absent or refused. */
export function readSplitSize(
  key: string | undefined,
  fallback: number,
  min: number,
  max: number,
): number {
  const initial = clampSplitSize(fallback, min, max);
  if (!key) return initial;
  try {
    const raw = localStorage.getItem(key);
    if (raw === null || raw.trim() === "") return initial;
    const stored = Number(raw);
    return Number.isFinite(stored) ? clampSplitSize(stored, min, max) : initial;
  } catch {
    return initial;
  }
}

/** writeSplitSize remembers a size or removes its key while tolerating refused storage. */
export function writeSplitSize(key: string | undefined, size: number | null): void {
  if (!key) return;
  try {
    if (size === null) localStorage.removeItem(key);
    else localStorage.setItem(key, String(size));
  } catch {
    return;
  }
}
