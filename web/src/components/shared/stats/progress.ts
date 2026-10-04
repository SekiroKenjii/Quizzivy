/**
 * ThresholdTone is the colour a result takes from how many got it right: the
 * item analysis's danger, warning and success.
 */
export type ThresholdTone = "danger" | "warning" | "success";

function clamp(value: number) {
  return Math.min(100, Math.max(0, value));
}

function ratio(value: number, max: number) {
  const share = (value / max) * 100;
  return max > 0 && !Number.isNaN(share) ? share : 0;
}

/**
 * percent is `value` as a whole percentage of `max`, rounded as `Math.round`
 * rounds and held between 0 and 100. A `max` of zero or less gives 0, as the
 * deck gives an assignment with nobody assigned.
 */
export function percent(value: number, max = 100): number {
  return clamp(Math.round(ratio(value, max)));
}

/**
 * thresholdTone is the deck's rule for an item-analysis bar: danger below 40
 * percent, warning below 65, success from 65.
 */
export function thresholdTone(percent: number): ThresholdTone {
  if (percent < 40) return "danger";
  if (percent < 65) return "warning";
  return "success";
}

/**
 * shares gives each of `values` its width on a track of `max`, as a
 * percentage to two decimal places and in order. A value that would run past
 * the end of the track takes what is left of it, so the widths never add up
 * to more than 100; a negative value, and any value when `max` is zero or
 * less, takes none.
 */
export function shares(values: readonly number[], max: number): number[] {
  let left = 100;
  return values.map((value) => {
    const width = Math.min(left, clamp(Math.round(ratio(value, max) * 100) / 100));
    left -= width;
    return width;
  });
}
