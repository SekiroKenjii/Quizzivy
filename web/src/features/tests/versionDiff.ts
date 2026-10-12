import type { TFunction } from "i18next";
import type { DiffChange } from "@/features/tests/api";
import type { PreviewMark } from "@/features/tests/components/StudentPreview";

type Counted = "added" | "removed" | "changed" | "answer";

const COUNTED: readonly Counted[] = ["added", "removed", "changed", "answer"];

/**
 * previewMarks maps the questions of the newer paper that a diff reports to
 * the mark the preview draws: "New" for an added question, "Changed" for one
 * whose words or answer changed. A removed question is not in that paper, and
 * the points total belongs to no question.
 */
export function previewMarks(
  changes: readonly DiffChange[],
  t: TFunction,
): Map<string, PreviewMark> {
  const marks = new Map<string, PreviewMark>();
  for (const change of changes) {
    if (change.questionId === undefined) continue;
    if (change.kind === "added") {
      marks.set(change.questionId, {
        tone: "added",
        label: t("tests.detail.compare.mark.added"),
      });
    } else if (
      (change.kind === "changed" || change.kind === "answer") &&
      !marks.has(change.questionId)
    ) {
      marks.set(change.questionId, {
        tone: "changed",
        label: t("tests.detail.compare.mark.changed"),
      });
    }
  }
  return marks;
}

/**
 * diffSummary is the line a version card shows when its teacher left no
 * change note (DG-66): how many questions were added, removed and changed
 * and how many answers were fixed against the version before it, joined
 * with " · ", or that no question changed.
 */
export function diffSummary(changes: readonly DiffChange[], t: TFunction): string {
  const counts = new Map<Counted, number>();
  for (const change of changes) {
    if (change.kind === "points") continue;
    counts.set(change.kind, (counts.get(change.kind) ?? 0) + 1);
  }
  const parts = COUNTED.filter((kind) => (counts.get(kind) ?? 0) > 0).map((kind) =>
    t(`tests.detail.history.summary.${kind}`, { count: counts.get(kind) }),
  );
  return parts.length === 0
    ? t("tests.detail.history.summary.none")
    : parts.join(" · ");
}
