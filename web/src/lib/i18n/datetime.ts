import { formatInTimeZone, toZonedTime } from "date-fns-tz";
import { vi, enUS } from "date-fns/locale";
import type { Locale as AppLocale } from "./index";

/**
 * Everything is stored and transported as UTC (§13.2). This module is the only
 * place a timezone is applied, so "store UTC, render Asia/Ho_Chi_Minh" is a
 * property of the codebase rather than a convention people remember.
 *
 * Do not call `new Date().toLocaleString()` anywhere else — it silently uses
 * the device timezone, which is wrong for a student travelling or a device with
 * a bad clock.
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

export function formatTime(utc: string | Date, locale: AppLocale = "vi") {
  return formatInTimeZone(utc, APP_TIME_ZONE, "HH:mm", {
    locale: dateFnsLocale[locale],
  });
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
export function formatDuration(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}
