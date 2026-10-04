function drawable(value: number) {
  return Number.isFinite(value) && value > 0 ? value : 0;
}

/**
 * barHeights gives each of `values` the height of its bar as a whole
 * percentage of the largest: the largest is 100 and the others in proportion,
 * rounded. A value that is negative or not a number draws nothing, and when
 * no value is above zero every bar is 0.
 */
export function barHeights(values: readonly number[]): number[] {
  const drawn = values.map(drawable);
  const top = Math.max(0, ...drawn);
  return drawn.map((value) => (top > 0 ? Math.round((value / top) * 100) : 0));
}

/**
 * labelShown reports whether the column at `index` draws its label when one
 * label in every `every` is drawn: the last column of each run of `every`, so
 * 2 keeps the odd indexes, as the deck does on a phone. An `every` below 1
 * counts as 1, which draws them all.
 */
export function labelShown(index: number, every: number): boolean {
  const step = Math.max(1, Math.floor(every) || 1);
  return index % step === step - 1;
}
