import type { Locale } from "@/lib/i18n";

/** credits writes a number of credits with the locale's grouping: "20,000", "20.000". */
export function credits(value: number, locale: Locale): string {
  return new Intl.NumberFormat(locale).format(value);
}

/**
 * resetsOn names the first day of the month after `month`, the first day of
 * a month counted in the centre's time zone (`YYYY-MM-DD`), as the deck
 * writes it: "1 November", "1 tháng 11". A calendar date carries no time, so
 * it is drawn in UTC and cannot move to a neighbouring day.
 */
export function resetsOn(month: string, locale: Locale): string {
  const [year, index] = month.split("-").map(Number);
  const next = Date.UTC(year ?? 1970, index ?? 1, 1);
  return new Intl.DateTimeFormat(locale === "vi" ? "vi" : "en-GB", {
    day: "numeric",
    month: "long",
    timeZone: "UTC",
  }).format(next);
}
