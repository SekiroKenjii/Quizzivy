import type { Locale } from "@/lib/i18n";
import { formatDate } from "@/lib/i18n/datetime";

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

const formats = new Map<Locale, Intl.RelativeTimeFormat>();

function formatFor(locale: Locale): Intl.RelativeTimeFormat {
  let format = formats.get(locale);
  if (format === undefined) {
    format = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
    formats.set(locale, format);
  }
  return format;
}

/**
 * relativeTime says how long ago `at` was in `locale`: null under a minute,
 * so the caller can say "Just now", then minutes, hours and days ("2 hours
 * ago", "yesterday"), and the date itself from a week back.
 */
export function relativeTime(at: string, now: number, locale: Locale): string | null {
  const elapsed = Math.max(0, now - Date.parse(at));
  if (elapsed < MINUTE) return null;
  const format = formatFor(locale);
  if (elapsed < HOUR) return format.format(-Math.floor(elapsed / MINUTE), "minute");
  if (elapsed < DAY) return format.format(-Math.floor(elapsed / HOUR), "hour");
  if (elapsed < 7 * DAY) return format.format(-Math.floor(elapsed / DAY), "day");
  return formatDate(at, locale);
}
