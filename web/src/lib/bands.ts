/** BANDS are the thirteen essay bands, 3 to 9 in half steps (DG-86). */
export const BANDS: readonly number[] = Array.from(
  { length: 13 },
  (_, index) => 3 + index / 2,
);

/** isBand says a value is one of the thirteen bands. */
export function isBand(value: number): boolean {
  return Number.isFinite(value) && value >= 3 && value <= 9 && (value * 2) % 1 === 0;
}

/**
 * bandOfCriteria is the band a rubric derives from its criteria, as the server
 * derives it (`shared/bands`, DG-61 interim): their mean rounded half up to
 * 0.5, so 6.25 is 6.5 and 6.125 is 6.0. It is null until every criterion has a
 * band, because a partial rubric has no band.
 */
export function bandOfCriteria(
  rubric: readonly { id: string }[],
  given: Readonly<Record<string, number>>,
): number | null {
  if (rubric.length === 0) return null;
  let sum = 0;
  for (const criterion of rubric) {
    const band = given[criterion.id];
    if (band === undefined) return null;
    sum += band;
  }
  return Math.floor((sum / rubric.length) * 2 + 0.5) / 2;
}

/** pointsOfBand is the score a band gives a question worth `points`, as the server stores it. */
export function pointsOfBand(band: number, points: number): number {
  return Math.round(((band * points) / 9) * 100) / 100;
}

const BAND_KEY = /^(?:Digit|Numpad)([3-9])$/;

/**
 * bandOfKey reads a band key by `event.code`, so the layout and Shift's
 * symbols do not matter: a digit 3 to 9 is that band and Shift+digit the half
 * above. Shift+9 and every other key are null.
 */
export function bandOfKey(
  event: Pick<KeyboardEvent, "code" | "shiftKey">,
): number | null {
  const digit = BAND_KEY.exec(event.code)?.[1];
  if (digit === undefined) return null;
  const band = Number(digit) + (event.shiftKey ? 0.5 : 0);
  return band <= 9 ? band : null;
}

/** formatBand writes a band with one decimal in the reader's locale: "6.0", or "6,0" in vi. */
export function formatBand(band: number, locale: string): string {
  return new Intl.NumberFormat(locale, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  }).format(band);
}
