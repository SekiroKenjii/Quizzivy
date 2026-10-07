import type { BarChartDatum } from "@/components/shared/charts/BarChart";
import { getDisplayTimeZone, appHour } from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";
import type { Dashboard } from "./api";

/** DashboardRange is the supported submission calendar range. */
export type DashboardRange = "7d" | "14d" | "30d";

/** dashboardRange reads a supported range without rewriting an absent or invalid URL. */
export function dashboardRange(search: string): DashboardRange {
  const value = new URLSearchParams(search).get("range");
  return value === "7d" || value === "30d" ? value : "14d";
}

/** dashboardRangeLocation changes only range and preserves repeated unrelated parameters and the hash. */
export function dashboardRangeLocation(
  location: Readonly<{ pathname: string; search: string; hash: string }>,
  range: DashboardRange,
) {
  const query = new URLSearchParams(location.search);
  query.set("range", range);
  return { pathname: location.pathname, search: `?${query}`, hash: location.hash };
}

/** dashboardGreeting follows the explicit app-zone morning, afternoon and evening boundaries. */
export function dashboardGreeting(now: Date): "morning" | "afternoon" | "evening" {
  const hour = appHour(now);
  if (hour >= 5 && hour < 12) return "morning";
  return hour >= 12 && hour < 18 ? "afternoon" : "evening";
}

/** dashboardLongDate formats the deck's long date in the staged app zone rather than the device zone. */
export function dashboardLongDate(now: Date, locale: Locale): string {
  const text = new Intl.DateTimeFormat(locale === "en" ? "en-GB" : locale, {
    timeZone: getDisplayTimeZone(),
    weekday: "long",
    day: "numeric",
    month: "long",
  }).format(now);
  if (locale === "en") {
    const parts = new Intl.DateTimeFormat("en-GB", {
      timeZone: getDisplayTimeZone(),
      weekday: "long",
      day: "numeric",
      month: "long",
    }).formatToParts(now);
    const value = (type: Intl.DateTimeFormatPartTypes) =>
      parts.find((part) => part.type === type)?.value ?? "";
    return `${value("weekday")}, ${value("day")} ${value("month")}`;
  }
  return text.charAt(0).toLocaleUpperCase(locale) + text.slice(1);
}

/** dashboardDestinations exposes only real positive-work destinations allowed by the current permissions. */
export function dashboardDestinations(
  data: Dashboard,
  grade: boolean,
  review: boolean,
) {
  const flagged = data.newestFlaggedAttempt;
  return {
    grade: grade && data.awaitingGrading > 0 ? "/teacher/grading" : null,
    flagged:
      review && data.flaggedAttempts > 0 && flagged?.assignmentId && flagged.attemptId
        ? `/teacher/assignments/${flagged.assignmentId}?tab=students&attempt=${flagged.attemptId}`
        : null,
    closing:
      (data.closingSoon ?? 0) > 0 && data.nextClosing?.id
        ? `/teacher/assignments/${data.nextClosing.id}?tab=students`
        : null,
    taking:
      data.takingNow.students > 0 && data.takingNow.assignmentId
        ? `/teacher/assignments/${data.takingNow.assignmentId}?tab=students`
        : null,
  };
}

/** dashboardBars retains calendar-date keys and every server-provided zero day. */
export function dashboardBars(
  days: Dashboard["submissions"]["days"],
  _locale: Locale,
  title: (count: number, date: string) => string,
): BarChartDatum[] {
  return days.map((day) => ({
    key: day.date,
    label: String(Number(day.date.slice(-2))),
    value: day.count,
    title: title(day.count, day.date.split("-").reverse().join("/")),
  }));
}

/** dashboardActivities gives indistinguishable activity duplicates distinct deterministic keys. */
export function dashboardActivities(rows: Dashboard["recentActivity"]) {
  const seen = new Map<string, number>();
  return rows.map((row) => {
    const identity = JSON.stringify([
      row.kind,
      row.at,
      row.studentName,
      row.subject,
      row.flagged,
    ]);
    const occurrence = seen.get(identity) ?? 0;
    seen.set(identity, occurrence + 1);
    return { ...row, key: `${identity}:${occurrence}` };
  });
}
