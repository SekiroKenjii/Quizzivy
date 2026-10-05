/** ThresholdTone names the semantic tone of an item-analysis result. */
export type ThresholdTone = "danger" | "warning" | "success";

function clamp(value: number) {
  return Math.min(100, Math.max(0, value));
}

function ratio(value: number, max: number) {
  const share = (value / max) * 100;
  return max > 0 && !Number.isNaN(share) ? share : 0;
}

/** percent rounds and clamps the percentage, returning zero for a nonpositive maximum. */
export function percent(value: number, max = 100): number {
  return clamp(Math.round(ratio(value, max)));
}

/** thresholdTone returns danger below forty percent, warning below sixty-five and success otherwise. */
export function thresholdTone(percent: number): ThresholdTone {
  if (percent < 40) return "danger";
  if (percent < 65) return "warning";
  return "success";
}

/** shares clamps ordered track shares to the remaining space and excludes invalid values. */
export function shares(values: readonly number[], max: number): number[] {
  let left = 100;
  return values.map((value) => {
    const width = Math.min(left, clamp(Math.round(ratio(value, max) * 100) / 100));
    left -= width;
    return width;
  });
}
