import type { Locale } from "@/lib/i18n";
import { formatDate } from "@/lib/i18n/datetime";

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/**
 * relativeTime says how long ago `at` was in `locale`: null under a minute,
 * so the caller can say "Just now", then minutes, hours and days ("2 hours
 * ago", "yesterday"), and the date itself from a week back.
 */
export function relativeTime(at: string, now: number, locale: Locale): string | null {
  const elapsed = Math.max(0, now - Date.parse(at));
  if (elapsed < MINUTE) return null;
  const format = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  if (elapsed < HOUR) return format.format(-Math.floor(elapsed / MINUTE), "minute");
  if (elapsed < DAY) return format.format(-Math.floor(elapsed / HOUR), "hour");
  if (elapsed < 7 * DAY) return format.format(-Math.floor(elapsed / DAY), "day");
  return formatDate(at, locale);
}
