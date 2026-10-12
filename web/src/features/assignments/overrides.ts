import type { MonitorRow } from "@/features/attempts/api";
import type { StudentOverride } from "./api";

const HANDED_IN = new Set<MonitorRow["state"]>(["submitted", "timed_out", "graded"]);

/** overridesKey is the query key of an assignment's student overrides. */
export function overridesKey(assignmentId: string) {
  return ["assignment-overrides", assignmentId] as const;
}

/**
 * attemptsAllowed is how many attempts `studentId` may make: the assignment's
 * `maxAttempts` plus the extra attempts their override already holds.
 */
export function attemptsAllowed(
  maxAttempts: number,
  overrides: readonly StudentOverride[],
  studentId: string,
): number {
  const stored = overrides.find((override) => override.studentId === studentId);
  return maxAttempts + (stored?.extraAttempts ?? 0);
}

/**
 * needsAnotherAttempt says whether reopening the assignment for `row` must
 * add an attempt: their latest attempt is handed in and it was their last
 * allowed one, so without one more they could not get back in (DG-163).
 */
export function needsAnotherAttempt(row: MonitorRow, allowed: number): boolean {
  return HANDED_IN.has(row.state) && (row.attemptNo ?? 0) >= allowed;
}

/**
 * keepsTimeAfter counts the students whose own close reaches past `at`:
 * closing the assignment early never takes that from them (T-R4.12).
 */
export function keepsTimeAfter(rows: readonly MonitorRow[], at: Date): number {
  return rows.filter(
    (row) =>
      row.extendedTo != null && new Date(row.extendedTo).getTime() > at.getTime(),
  ).length;
}
