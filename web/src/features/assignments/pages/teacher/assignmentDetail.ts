import type { AttemptReview, MonitorRow } from "@/features/attempts/api";
import { isHandedIn } from "@/features/attempts/api";
import { fold } from "@/lib/fold";

/** DetailTab names the assignment detail's three URL-controlled panels. */
export type DetailTab = "students" | "questions" | "settings";
/** RosterFilter names the supported paper filters independently of the detail tab. */
export type RosterFilter =
  "all" | "submitted" | "inProgress" | "pending" | "flagged" | "notStarted";

/** ProgressGroup names the roster groups the progress card counts and filters by, in its legend's order. */
export type ProgressGroup = "submitted" | "inProgress" | "notStarted" | "flagged";

/** PROGRESS_GROUPS lists the progress card's groups in the order its legend draws them. */
export const PROGRESS_GROUPS: readonly ProgressGroup[] = [
  "submitted",
  "inProgress",
  "notStarted",
  "flagged",
];

/** detailTab reads a supported panel and defaults to Students. */
export function detailTab(value: string | null): DetailTab {
  return value === "questions" || value === "settings" ? value : "students";
}

/** rosterFilter reads a supported paper filter and defaults to all students. */
export function rosterFilter(value: string | null): RosterFilter {
  return value === "submitted" ||
    value === "inProgress" ||
    value === "pending" ||
    value === "flagged" ||
    value === "notStarted"
    ? value
    : "all";
}

/** pendingAnswers counts manual answers only on handed-in papers. */
export function pendingAnswers(row: MonitorRow): number {
  return isHandedIn(row.state) ? (row.score?.pendingManual ?? 0) : 0;
}

/**
 * assignmentStats derives all figures from the complete monitor: each progress
 * group counts the rows its roster filter shows, and the average is the rounded
 * mean of graded positive-denominator ratios.
 */
export function assignmentStats(rows: readonly MonitorRow[]) {
  const scored = rows.filter(
    (row) => row.state === "graded" && row.score != null && row.score.total > 0,
  );
  const count = (group: ProgressGroup) =>
    rows.filter((row) => rosterMatches(row, group, "")).length;
  return {
    submitted: count("submitted"),
    inProgress: count("inProgress"),
    notStarted: count("notStarted"),
    flagged: count("flagged"),
    total: rows.length,
    pending: rows.reduce((sum, row) => sum + pendingAnswers(row), 0),
    average:
      scored.length === 0
        ? null
        : Math.round(
            scored.reduce(
              (sum, row) => sum + (row.score!.earned / row.score!.total) * 100,
              0,
            ) / scored.length,
          ),
  };
}

/** rosterMatches applies the existing paper-filter semantics and accent-insensitive Vietnamese name search. */
export function rosterMatches(
  row: MonitorRow,
  filter: RosterFilter,
  query: string,
): boolean {
  const search = fold(query.trim());
  if (search !== "" && !fold(row.fullName).includes(search)) return false;
  switch (filter) {
    case "submitted":
      return isHandedIn(row.state);
    case "inProgress":
      return row.state === "in_progress";
    case "pending":
      return pendingAnswers(row) > 0;
    case "flagged":
      return row.flagged === true && row.state !== "voided";
    case "notStarted":
      return row.state === "not_started";
    case "all":
      return true;
  }
}

/** orderedRoster sorts names in Vietnamese with immutable student identity as the tie-breaker. */
export function orderedRoster(rows: readonly MonitorRow[]): MonitorRow[] {
  const collator = new Intl.Collator("vi", { sensitivity: "base", numeric: true });
  return [...rows].sort(
    (a, b) =>
      collator.compare(a.fullName, b.fullName) ||
      a.studentId.localeCompare(b.studentId),
  );
}

/** elapsedMinutes reports actual handed-in elapsed time and leaves absent timestamps unknown. */
export function elapsedMinutes(
  row: Pick<MonitorRow, "startedAt" | "submittedAt">,
): number | null {
  if (!row.startedAt || !row.submittedAt) return null;
  return Math.max(
    0,
    Math.round(
      (new Date(row.submittedAt).getTime() - new Date(row.startedAt).getTime()) /
        60_000,
    ),
  );
}

/** reviewMatches binds a review to the requested assignment, attempt and student before any private presentation. */
export function reviewMatches(
  data: AttemptReview | undefined,
  assignmentId: string,
  attemptId: string,
): data is AttemptReview {
  return (
    data !== undefined &&
    data.attempt.id === attemptId &&
    data.attempt.assignmentId === assignmentId &&
    data.student.id === data.attempt.studentId
  );
}

/** firstPendingPaper selects the first real handed-in paper with manual answers in stable roster order. */
export function firstPendingPaper(rows: readonly MonitorRow[]): MonitorRow | undefined {
  return orderedRoster(rows).find(
    (row) => Boolean(row.attemptId) && pendingAnswers(row) > 0,
  );
}
