import { listAttempts, type AttemptListRow, type GradingQueueItem } from "../../api";

/** scanGradingCandidates exhausts both settled statuses without claiming a concurrent snapshot. */
export async function scanGradingCandidates(
  signal: AbortSignal,
  assignment: string,
  student: string,
) {
  const found = new Map<string, AttemptListRow>();
  for (const status of ["submitted", "timed_out"] as const) {
    const rows = await scanStatus(status, signal);
    for (const item of rows) {
      if (
        (!assignment || item.assignmentId === assignment) &&
        (!student || item.studentId === student)
      )
        found.set(item.id, item);
    }
  }
  return [...found.values()];
}

async function scanStatus(status: "submitted" | "timed_out", signal: AbortSignal) {
  const rows: AttemptListRow[] = [];
  let page = 1;
  let last = 1;
  do {
    signal.throwIfAborted();
    const result = await listAttempts(
      { status, pendingGrading: false, limit: 100, page },
      signal,
    );
    signal.throwIfAborted();
    if (
      !Number.isFinite(result.pageSize) ||
      result.pageSize <= 0 ||
      result.page !== page
    )
      throw new Error("Invalid recovery page");
    rows.push(...result.items);
    last = Math.ceil(result.total / result.pageSize);
    if (page < last && result.items.length === 0)
      throw new Error("Incomplete recovery page");
    page += 1;
  } while (page <= last);
  return rows;
}

/** gradingItemKey uses immutable paper and frozen-question identities. */
export function gradingItemKey(
  item: Pick<GradingQueueItem, "attemptId" | "questionId">,
) {
  return `${item.attemptId}:${item.questionId}`;
}

/** gradingGroupKey keeps repeated names and reused frozen questions in separate groups. */
export function gradingGroupKey(item: GradingQueueItem, mode: "student" | "question") {
  return mode === "student"
    ? item.studentId
    : `${item.assignmentId}:${item.questionId}`;
}

/**
 * mergeQueueOrder keeps every item where the queue first showed it and places
 * each newly returned key right after the key the server returned before it,
 * so a graded answer that leaves the server's pending list keeps its place.
 */
export function mergeQueueOrder(
  order: readonly string[],
  incoming: readonly string[],
): string[] {
  const result = [...order];
  const placed = new Set(result);
  let anchor = -1;
  for (const key of incoming) {
    if (placed.has(key)) {
      anchor = result.indexOf(key);
      continue;
    }
    result.splice(anchor + 1, 0, key);
    placed.add(key);
    anchor += 1;
  }
  return result;
}

/** scoreOptions returns half-point choices only when all choices fit the nine keyboard slots. */
export function scoreOptions(max: number) {
  if (max > 4 || max < 0) return [];
  const choices = Array.from(
    { length: Math.floor(max * 2) + 1 },
    (_, index) => index / 2,
  );
  if (choices.at(-1) !== max) choices.push(max);
  return choices.length <= 9 ? choices : [];
}
