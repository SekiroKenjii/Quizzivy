import type { TFunction } from "i18next";
import type { DiffChange } from "@/features/tests/api";

type Counted = "added" | "removed" | "changed" | "answer";

const COUNTED: readonly Counted[] = ["added", "removed", "changed", "answer"];

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
