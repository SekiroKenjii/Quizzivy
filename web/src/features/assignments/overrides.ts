import type { MonitorRow } from "@/features/attempts/api";

const HANDED_IN = new Set<MonitorRow["state"]>(["submitted", "timed_out", "graded"]);

/**
 * needsAnotherAttempt says whether reopening the assignment for `row` must
 * add an attempt: their latest attempt is handed in and it was their last
 * allowed one, so without one more they could not get back in (DG-163).
 */
export function needsAnotherAttempt(row: MonitorRow, maxAttempts: number): boolean {
  return HANDED_IN.has(row.state) && (row.attemptNo ?? 0) >= maxAttempts;
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
