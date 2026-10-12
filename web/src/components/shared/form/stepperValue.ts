/** clampTo keeps `value` between `min` and `max`, both included. */
export function clampTo(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

/** digitsOf keeps the digits of what was typed and drops everything else. */
export function digitsOf(text: string): string {
  return text.replaceAll(/\D/g, "");
}

/**
 * typedValue reads the whole number a draft holds and clamps it to `min` and
 * `max`, without snapping it to any step. A draft that holds no number, an
 * empty one included, is null.
 */
export function typedValue(draft: string, min: number, max: number): number | null {
  if (!/^\d+$/.test(draft)) return null;
  return clampTo(Number(draft), min, max);
}

/**
 * keyTarget is where a key takes a stepper that stands at `current`: one
 * `step` up or down for the arrows, `min` for Home and `max` for End, not yet
 * clamped. Any other key is null.
 */
export function keyTarget(
  key: string,
  current: number,
  step: number,
  min: number,
  max: number,
): number | null {
  switch (key) {
    case "ArrowUp":
      return current + step;
    case "ArrowDown":
      return current - step;
    case "Home":
      return min;
    case "End":
      return max;
    default:
      return null;
  }
}
