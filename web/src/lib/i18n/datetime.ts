import { formatInTimeZone, fromZonedTime, toZonedTime } from "date-fns-tz";
import { vi, enUS } from "date-fns/locale";
import type { Locale as AppLocale } from "./index";

/**
 * Everything is stored and transported as UTC (§13.2). This module is the only
 * place a timezone is applied, so "store UTC, render Asia/Ho_Chi_Minh" is a
 * property of the codebase rather than a convention people remember.
 */
export const APP_TIME_ZONE = "Asia/Ho_Chi_Minh";

const dateFnsLocale = { vi, en: enUS } as const;

export function formatDateTime(utc: string | Date, locale: AppLocale = "vi") {
  return formatInTimeZone(utc, APP_TIME_ZONE, "HH:mm, dd/MM/yyyy", {
    locale: dateFnsLocale[locale],
  });
}

export function formatDate(utc: string | Date, locale: AppLocale = "vi") {
  return formatInTimeZone(utc, APP_TIME_ZONE, "dd/MM/yyyy", {
    locale: dateFnsLocale[locale],
  });
}

/** G-09's moment: "08:00 · Thứ hai, 07/09", weekday capitalised as the deck writes it. */
export function formatMoment(utc: string | Date, locale: AppLocale = "vi") {
  const text = formatInTimeZone(utc, APP_TIME_ZONE, "HH:mm · EEEE, dd/MM", {
    locale: dateFnsLocale[locale],
  });
  const sentence = locale === "vi" ? text.toLocaleLowerCase("vi") : text;
  return sentence.replace(
    /· (\p{L})/u,
    (_, first: string) => `· ${first.toUpperCase()}`,
  );
}

/** "26/08" */
export function shortDate(utc: string | Date) {
  return formatInTimeZone(utc, APP_TIME_ZONE, "dd/MM");
}

export function sameAppDay(a: string | Date, b: string | Date): boolean {
  return (
    formatInTimeZone(a, APP_TIME_ZONE, "yyyy-MM-dd") ===
    formatInTimeZone(b, APP_TIME_ZONE, "yyyy-MM-dd")
  );
}

/** appHour is the hour of the day, 0 to 23, in the app's zone. */
export function appHour(utc: string | Date): number {
  return Number(formatInTimeZone(utc, APP_TIME_ZONE, "H"));
}

/**
 * appDaysUntil counts calendar days in the app's zone from the day of `now`
 * to the day of `utc`: 0 on the same day, 1 on the next, negative for a day
 * already past.
 */
export function appDaysUntil(utc: string | Date, now: string | Date): number {
  const day = (moment: string | Date) => {
    const [y = 0, m = 1, d = 1] = formatInTimeZone(moment, APP_TIME_ZONE, "yyyy-M-d")
      .split("-")
      .map(Number);
    return Date.UTC(y, m - 1, d);
  };
  return Math.round((day(utc) - day(now)) / 86_400_000);
}

/** weekdayShort is "T5" or "CN" in Vietnamese and "Thu" in English. */
export function weekdayShort(utc: string | Date, locale: AppLocale = "vi") {
  return formatInTimeZone(utc, APP_TIME_ZONE, locale === "vi" ? "EEEEE" : "EEE", {
    locale: dateFnsLocale[locale],
  });
}

/**
 * weekdayName is the weekday as it reads inside a sentence: "thứ năm" in
 * Vietnamese, "Thursday" in English.
 */
export function weekdayName(utc: string | Date, locale: AppLocale = "vi") {
  const text = formatInTimeZone(utc, APP_TIME_ZONE, "EEEE", {
    locale: dateFnsLocale[locale],
  });
  return locale === "vi" ? text.toLocaleLowerCase("vi") : text;
}

/** dayOfMonth is the day number without a leading zero: "5", "25". */
export function dayOfMonth(utc: string | Date) {
  return formatInTimeZone(utc, APP_TIME_ZONE, "d");
}

/** dayMonth is a date without its year: "05/09" in Vietnamese, "5 Sep" in English. */
export function dayMonth(utc: string | Date, locale: AppLocale = "vi") {
  return formatInTimeZone(utc, APP_TIME_ZONE, locale === "vi" ? "dd/MM" : "d MMM", {
    locale: dateFnsLocale[locale],
  });
}

/**
 * dayDate is a short weekday and date: "Thứ 6, 19/09" in Vietnamese, "Fri 19
 * Sep" in English.
 */
export function dayDate(utc: string | Date, locale: AppLocale = "vi") {
  return formatInTimeZone(
    utc,
    APP_TIME_ZONE,
    locale === "vi" ? "EEE, dd/MM" : "EEE d MMM",
    { locale: dateFnsLocale[locale] },
  );
}

/** "Thứ hai, 01/09" -- the weekday the deck writes on upcoming rows. */
export function weekdayDate(
  utc: string | Date,
  locale: AppLocale = "vi",
  year = false,
) {
  const text = formatInTimeZone(
    utc,
    APP_TIME_ZONE,
    year ? "EEEE, dd/MM/yyyy" : "EEEE, dd/MM",
    {
      locale: dateFnsLocale[locale],
    },
  );
  const sentence = locale === "vi" ? text.toLocaleLowerCase("vi") : text;
  return sentence.charAt(0).toUpperCase() + sentence.slice(1);
}

/** An audio length as m:ss ("1:50"), the shape read off a player; "—" when unknown. */
export function audioLength(ms: number | null | undefined): string {
  if (ms == null || ms <= 0) return "—";
  const total = Math.round(ms / 1000);
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, "0")}`;
}

export function formatTime(utc: string | Date, locale: AppLocale = "vi") {
  return formatInTimeZone(utc, APP_TIME_ZONE, "HH:mm", {
    locale: dateFnsLocale[locale],
  });
}

/** clockTime is "22:30" on the day of `now`, and "22:30, 02/10" on any other day. */
export function clockTime(utc: string | Date, now: string | Date = new Date()) {
  const time = formatInTimeZone(utc, APP_TIME_ZONE, "HH:mm");
  return sameAppDay(utc, now) ? time : `${time}, ${shortDate(utc)}`;
}

/**
 * "2 giờ trước", "hôm qua", then a plain date -- the deck's A-03 column.
 *
 * Relative wording earns its keep for about a week; past that "3 tuần trước" is
 * a worse answer than the date, because the teacher is looking for a specific
 * test they remember by when they wrote it.
 */
export function formatRelative(utc: string | Date, locale: AppLocale = "vi") {
  const then = new Date(utc).getTime();
  const seconds = Math.round((then - Date.now()) / 1000);
  const days = Math.round(seconds / 86_400);

  if (Math.abs(days) > 6) return formatDate(utc, locale);

  const relative = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  const minutes = Math.round(seconds / 60);
  if (Math.abs(minutes) < 1) return relative.format(0, "minute");
  if (Math.abs(minutes) < 60) return relative.format(minutes, "minute");

  const hours = Math.round(minutes / 60);
  if (Math.abs(hours) < 24) return relative.format(hours, "hour");
  const result = relative.format(days, "day");
  return locale === "vi" ? result.toLocaleLowerCase("vi") : result;
}

/** The two halves of a `datetime-local` field, both pinned to APP_TIME_ZONE. */
export function toDateTimeInput(utc: string | Date): string {
  return formatInTimeZone(utc, APP_TIME_ZONE, "yyyy-MM-dd'T'HH:mm");
}

export function fromDateTimeInput(value: string): Date {
  return fromZonedTime(value, APP_TIME_ZONE);
}

/** The same instant, expressed in the app's timezone. For date maths in the UI. */
export function inAppZone(utc: string | Date) {
  return toZonedTime(utc, APP_TIME_ZONE);
}

/**
 * Remaining time as mm:ss (or h:mm:ss past an hour), for the test-taking timer.
 * Takes a millisecond count rather than two dates, because the take-test store
 * computes remaining from the server clock offset, never from Date.now() alone.
 */
export function countdown(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

/** compactMoment formats a table timestamp without repeating the current year. */
export function compactMoment(utc: string | Date) {
  return formatInTimeZone(utc, APP_TIME_ZONE, "HH:mm · dd/MM");
}
