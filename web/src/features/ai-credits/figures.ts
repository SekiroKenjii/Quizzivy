import type { AiTeacherMonth, AiTeacherStatus } from "@/features/ai-credits/api";
import type { Locale } from "@/lib/i18n";

/** BarTone is the colour of a usage bar: the accent, then warning, then danger. */
export type BarTone = "accent" | "warning" | "danger";

/**
 * poolTone is the pool bar's colour for `used` of `total` credits: the accent
 * under 75%, warning from 75% and danger from 90%. The shares are compared
 * exactly, so 74.9% is still the accent and 89.9% still warning. A pool with
 * nothing in it is danger as soon as anything is used.
 */
export function poolTone(used: number, total: number): BarTone {
  if (total <= 0) return used > 0 ? "danger" : "accent";
  if (used * 10 >= total * 9) return "danger";
  if (used * 4 >= total * 3) return "warning";
  return "accent";
}

/**
 * projectionAt is where the projection's tick sits along the pool bar, as a
 * percentage kept inside the track, or null when there is nothing to mark: no
 * pool, or no projected use. The projection itself is the server's `onPace`.
 */
export function projectionAt(onPace: number, total: number): number | null {
  if (total <= 0 || onPace <= 0) return null;
  return Math.min(99.5, (onPace / total) * 100);
}

/** paceFigure is the projection as the pace fact shows it: to the nearest hundred once it is that large. */
export function paceFigure(onPace: number): number {
  return onPace < 100 ? onPace : Math.round(onPace / 100) * 100;
}

const TEACHER_TONES: Record<AiTeacherStatus, BarTone> = {
  on_track: "accent",
  near_limit: "warning",
  at_limit: "danger",
  paused: "danger",
};

/** teacherTone is the colour of a teacher's bar for their standing. */
export function teacherTone(status: AiTeacherStatus): BarTone {
  return TEACHER_TONES[status];
}

function shareOf(teacher: AiTeacherMonth): number {
  if (teacher.allowance > 0) return teacher.used / teacher.allowance;
  return teacher.used > 0 ? Number.POSITIVE_INFINITY : 0;
}

/**
 * byUse orders teachers by the share of their allowance used, the most used
 * first, then by name, so two teachers at the same share keep a steady order.
 * It returns a new array.
 */
export function byUse(
  teachers: readonly AiTeacherMonth[],
  locale: Locale,
): AiTeacherMonth[] {
  return [...teachers].sort((a, b) => {
    const gap = shareOf(b) - shareOf(a);
    if (gap !== 0 && !Number.isNaN(gap)) return gap;
    return a.name.localeCompare(b.name, locale) || a.userId.localeCompare(b.userId);
  });
}

function dateParts(month: string): { year: number; index: number } {
  const [year, index] = month.split("-").map(Number);
  return { year: year ?? 1970, index: (index ?? 1) - 1 };
}

function intlLocale(locale: Locale): string {
  return locale === "vi" ? "vi" : "en-GB";
}

/** monthName names the calendar month `month` (`YYYY-MM-DD`) falls in: "October", "tháng 10". */
export function monthName(month: string, locale: Locale): string {
  const { year, index } = dateParts(month);
  return new Intl.DateTimeFormat(intlLocale(locale), {
    month: "long",
    timeZone: "UTC",
  }).format(Date.UTC(year, index, 1));
}

/** lastDayOf writes the last day of the month `month` falls in, as the deck does: "31 October", "31 tháng 10". */
export function lastDayOf(month: string, locale: Locale): string {
  const { year, index } = dateParts(month);
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "long",
    timeZone: "UTC",
  }).format(Date.UTC(year, index + 1, 0));
}

/**
 * withOption returns `options` with `value` among them in numeric order, for a
 * rule the maintenance command set to a figure the buttons do not offer, so
 * the control still shows what is in force.
 */
export function withOption(
  options: readonly number[],
  value: number,
): readonly number[] {
  if (options.includes(value)) return options;
  return [...options, value].sort((a, b) => a - b);
}
