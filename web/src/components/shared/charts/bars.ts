function drawable(value: number) {
  return Number.isFinite(value) && value > 0 ? value : 0;
}

/** barHeights rounds each valid value relative to the largest and returns zero when none is positive. */
export function barHeights(values: readonly number[]): number[] {
  const drawn = values.map(drawable);
  const top = Math.max(0, ...drawn);
  return drawn.map((value) => (top > 0 ? Math.round((value / top) * 100) : 0));
}

/** labelShown selects the last label of each cadence, treating a cadence below one as one. */
export function labelShown(index: number, every: number): boolean {
  const step = Math.max(1, Math.floor(every) || 1);
  return index % step === step - 1;
}
