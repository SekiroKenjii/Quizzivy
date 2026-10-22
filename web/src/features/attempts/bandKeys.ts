import type { KeyboardEvent } from "react";
import { bandOfKey } from "@/lib/bands";
import type { components } from "@/lib/api/schema";
import type { EssayMark } from "./essayMark";

type Criterion = components["schemas"]["RubricCriterion"];

/**
 * bandKeyEvent reads a band key from a keydown that carries no modifier but
 * Shift, so Ctrl+5 or Alt+5 keep their own meaning.
 */
export function bandKeyEvent(
  event: Pick<KeyboardEvent, "code" | "shiftKey" | "altKey" | "ctrlKey" | "metaKey"> & {
    isComposing?: boolean;
    nativeEvent?: { isComposing: boolean };
  },
): number | null {
  if (event.altKey || event.ctrlKey || event.metaKey) return null;
  if (event.isComposing || event.nativeEvent?.isComposing) return null;
  return bandOfKey(event);
}

/**
 * nextCriterion is the criterion the page's band keys set: the one whose scale
 * last took focus, else the first without a band, else the last. Null when the
 * essay has no rubric, so the key sets the band itself.
 */
export function nextCriterion(
  rubric: readonly Criterion[],
  mark: EssayMark,
  active: string | null,
): string | null {
  if (rubric.length === 0) return null;
  if (active !== null && rubric.some((criterion) => criterion.id === active))
    return active;
  return (
    rubric.find((criterion) => mark.criteria[criterion.id] === undefined)?.id ??
    rubric.at(-1)!.id
  );
}

/**
 * focusAfter moves focus, after a band key set a criterion, to the next
 * criterion's scale (its chosen band, else its first), so a rubric is marked
 * with one key per criterion. The last criterion keeps focus.
 */
export function focusAfter(
  root: HTMLElement | null,
  rubric: readonly Criterion[],
  criterionId: string,
) {
  const at = rubric.findIndex((criterion) => criterion.id === criterionId);
  const next = rubric[at + 1];
  if (!root || !next) return;
  const scale = `[data-score^="${next.id}:"]`;
  (
    root.querySelector<HTMLElement>(`${scale}[aria-pressed="true"]`) ??
    root.querySelector<HTMLElement>(scale)
  )?.focus();
}
