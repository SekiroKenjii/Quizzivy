import type { TFunction } from "i18next";
import { formatInTimeZone } from "date-fns-tz";
import { vi, enUS } from "date-fns/locale";
import { APP_TIME_ZONE, formatTime } from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";

/**
 * The words on S-03's cards. Every one is about a moment in the app's
 * timezone, never the device's (§13.2), so they all go through datetime.ts's
 * zone rather than `Date` methods.
 */

/** "Minh" from "Nguyễn Đức Minh": a Vietnamese given name is the last word. */
export function givenName(fullName: string): string {
  const parts = fullName.trim().split(/\s+/);
  return parts[parts.length - 1] ?? "";
}

/** "Còn 3 giờ": whole units, the largest that fits, never "0 phút". */
export function timeLeft(closesAt: string, now: Date, t: TFunction): string {
  const ms = new Date(closesAt).getTime() - now.getTime();
  const minutes = Math.max(1, Math.round(ms / 60_000));
  if (minutes < 60) return t("student.leftMinutes", { count: minutes });
  const hours = Math.round(minutes / 60);
  if (hours < 24) return t("student.leftHours", { count: hours });
  return t("student.leftDays", { count: Math.round(hours / 24) });
}

export function sameAppDay(a: string | Date, b: string | Date): boolean {
  return (
    formatInTimeZone(a, APP_TIME_ZONE, "yyyy-MM-dd") ===
    formatInTimeZone(b, APP_TIME_ZONE, "yyyy-MM-dd")
  );
}

/** "26/08" */
export function shortDate(utc: string | Date): string {
  return formatInTimeZone(utc, APP_TIME_ZONE, "dd/MM");
}

/** "Thứ hai, 01/09" -- the weekday the deck writes on upcoming rows. */
export function weekdayDate(utc: string | Date, locale: Locale): string {
  const text = formatInTimeZone(utc, APP_TIME_ZONE, "EEEE, dd/MM", {
    locale: locale === "vi" ? vi : enUS,
  });
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** "Đóng lúc 21:00 hôm nay" or "Đóng lúc 21:00 · 29/08". */
export function closesLine(closesAt: string, now: Date, t: TFunction): string {
  const time = formatTime(closesAt);
  return sameAppDay(closesAt, now)
    ? t("student.closesToday", { time })
    : t("student.closesAt", { time, date: shortDate(closesAt) });
}

/** "27/30", with the decimals the numbers actually have. */
export function scoreText(
  earned: number,
  total: number,
  locale: Locale,
  t: TFunction,
): string {
  const n = new Intl.NumberFormat(locale, { maximumFractionDigits: 2 });
  return t("student.score", { earned: n.format(earned), total: n.format(total) });
}
