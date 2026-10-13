import {
  listAttempts,
  type AttemptListRow,
  type AttemptReview,
  type AttemptScore,
  type GradingQueueItem,
} from "../../api";

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

/**
 * nextOpenItem returns the first item after index `from` in the shown order,
 * wrapping round to the start, for which `open` holds, or null when none does.
 */
export function nextOpenItem<T>(
  items: readonly T[],
  from: number,
  open: (item: T) => boolean,
): T | null {
  for (let offset = 1; offset <= items.length; offset += 1) {
    const item = items[(from + offset) % items.length]!;
    if (open(item)) return item;
  }
  return null;
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

/** ReadyGroup is one assignment's papers waiting for an explicit Finish. */
export interface ReadyGroup {
  assignmentId: string;
  title: string;
  earliest: string | null;
  rows: AttemptListRow[];
}

function handedIn(row: AttemptListRow) {
  return row.submittedAt ? Date.parse(row.submittedAt) : Number.POSITIVE_INFINITY;
}

function byHandIn(a: AttemptListRow, b: AttemptListRow) {
  const left = handedIn(a);
  const right = handedIn(b);
  if (left === right) return 0;
  return left < right ? -1 : 1;
}

/**
 * groupReadyPapers groups papers by assignment and orders the groups and the
 * rows inside each by the oldest hand-in first; a paper without a hand-in time
 * goes last, and ties keep the order the scan returned.
 */
export function groupReadyPapers(rows: readonly AttemptListRow[]): ReadyGroup[] {
  const groups = new Map<string, ReadyGroup>();
  for (const row of [...rows].sort(byHandIn)) {
    const group = groups.get(row.assignmentId);
    if (group) group.rows.push(row);
    else
      groups.set(row.assignmentId, {
        assignmentId: row.assignmentId,
        title: row.testTitle,
        earliest: row.submittedAt ?? null,
        rows: [row],
      });
  }
  return [...groups.values()];
}

/** PaperKind is how a paper was marked: blank, automatically, or by hand. */
export type PaperKind =
  { kind: "blank" } | { kind: "auto" } | { kind: "hand"; marked: number };

/** PaperCheck is what a ready paper's check shows, read from a fresh review. */
export interface PaperCheck {
  score: AttemptScore | null;
  answered: number;
  questions: number;
  marking: PaperKind;
  flagged: boolean;
  focusLosses: number;
  finishable: boolean;
}

/**
 * paperCheck reads a review: the score, the questions with a non-null answer,
 * how the paper was marked (blank when nothing was answered, by hand when any
 * answer needs hand marking, otherwise automatically), its flag, and whether
 * Finish may be offered, which needs a handed-in paper whose pendingManual is
 * exactly 0.
 */
export function paperCheck(review: AttemptReview): PaperCheck {
  const entries = review.questions.map((question) => review.answers[question.id]);
  const answered = entries.filter((entry) => entry?.answer != null).length;
  const marked = entries.filter((entry) => entry?.requiresManual === true).length;
  const score = review.attempt.score ?? null;
  let marking: PaperKind = { kind: "auto" };
  if (answered === 0) marking = { kind: "blank" };
  else if (marked > 0) marking = { kind: "hand", marked };
  return {
    score,
    answered,
    questions: review.questions.length,
    marking,
    flagged: review.attempt.integrity?.flagged === true,
    focusLosses: review.attempt.integrity?.focusLossCount ?? 0,
    finishable:
      score?.pendingManual === 0 &&
      (review.attempt.status === "submitted" || review.attempt.status === "timed_out"),
  };
}
