import type { DateTimeMode } from "./DateTimeField";
const pad = (value: number) => String(value).padStart(2, "0");
/** parseDateTime reads a wall-clock key without parsing UTC or applying a timezone conversion. */
export function parseDateTime(
  value: string,
  mode: DateTimeMode,
  now = new Date(),
): Date | null {
  const patterns = {
    time: /^(\d{2}):(\d{2})$/,
    date: /^(\d{4})-(\d{2})-(\d{2})$/,
    datetime: /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/,
  };
  const pattern = patterns[mode];
  const parts = pattern.exec(value);
  if (!parts) return null;
  const time = mode === "time";
  const y = time ? now.getFullYear() : Number(parts[1]);
  const m = time ? now.getMonth() : Number(parts[2]) - 1;
  const day = time ? now.getDate() : Number(parts[3]);
  const h = time ? Number(parts[1]) : Number(parts[4] ?? 0);
  const min = time ? Number(parts[2]) : Number(parts[5] ?? 0);
  if (m < 0 || m > 11 || day < 1 || day > 31 || h < 0 || h > 23 || min < 0 || min > 59)
    return null;
  const date = new Date(y, m, day, h, min);
  date.setFullYear(y);
  return date.getFullYear() === y && date.getMonth() === m && date.getDate() === day
    ? date
    : null;
}
/** serializeDateTime writes the selected calendar and clock components as the caller's wall-clock key. */
export function serializeDateTime(date: Date, mode: DateTimeMode): string {
  const time = `${pad(date.getHours())}:${pad(date.getMinutes())}`;
  const day = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  if (mode === "time") return time;
  if (mode === "date") return day;
  return `${day}T${time}`;
}
/** dateTimeLabel formats the wall-clock selection in the reader's language. */
export function dateTimeLabel(date: Date, mode: DateTimeMode, locale: string): string {
  if (mode === "time") return serializeDateTime(date, "time");
  const day = new Intl.DateTimeFormat(locale === "en" ? "en-GB" : "vi-VN", {
    weekday: "short",
    day: "numeric",
    month: "short",
    ...(mode === "date" ? { year: "numeric" as const } : {}),
  }).format(date);
  return mode === "datetime" ? `${day}, ${serializeDateTime(date, "time")}` : day;
}
/** pickerStart retains a valid controlled value or opens an empty picker at the current full hour. */
export function pickerStart(value: string, mode: DateTimeMode, now = new Date()): Date {
  const parsed = parseDateTime(value, mode, now);
  if (parsed) return parsed;
  const full = new Date(now);
  full.setMinutes(0, 0, 0);
  return full;
}
/** minuteStepValue bounds the minute increment to a positive whole number within an hour. */
export function minuteStepValue(step: number): number {
  return Number.isInteger(step) && step > 0 && step <= 60 ? step : 5;
}
/** bumpHour wraps the clock hour while preserving the date and minute. */
export function bumpHour(date: Date, delta: number): Date {
  const next = new Date(date);
  next.setHours((date.getHours() + delta + 24) % 24);
  return next;
}
/** bumpMinute snaps to the nearest step before moving and wraps without carrying into the hour. */
export function bumpMinute(date: Date, delta: number, step: number): Date {
  const increment = minuteStepValue(step);
  const next = new Date(date);
  next.setMinutes(
    (Math.round(date.getMinutes() / increment) * increment + delta * increment + 60) %
      60,
  );
  return next;
}
/** chooseDay replaces the calendar day while preserving the draft's clock. */
export function chooseDay(draft: Date, day: Date): Date {
  return new Date(
    day.getFullYear(),
    day.getMonth(),
    day.getDate(),
    draft.getHours(),
    draft.getMinutes(),
  );
}
