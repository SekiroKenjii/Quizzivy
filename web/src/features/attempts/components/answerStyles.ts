/** The deck's `.option.is-correct` / `.is-wrong` and `.qdot` states, as Tailwind. */
export const OPTION = {
  base: "flex items-start gap-3 rounded-lg border px-4 py-2.5",
  selected:
    "border-foreground bg-accent shadow-[inset_0_0_0_1px_var(--color-foreground)]",
  correct:
    "border-[color-mix(in_oklab,var(--success)_45%,transparent)] bg-[color-mix(in_oklab,var(--success)_8%,var(--background))]",
  wrong:
    "border-[color-mix(in_oklab,var(--destructive)_40%,transparent)] bg-[color-mix(in_oklab,var(--destructive)_6%,var(--background))]",
  key: "text-muted-foreground grid size-6 shrink-0 place-content-center rounded-sm border text-xs font-semibold",
} as const;

export const DOT = {
  base: "bg-background grid place-content-center rounded-md border text-xs tabular-nums",
  correct:
    "bg-[color-mix(in_oklab,var(--success)_14%,var(--background))] text-[color-mix(in_oklab,var(--success)_70%,var(--foreground))] border-[color-mix(in_oklab,var(--success)_30%,transparent)]",
  wrong:
    "bg-[color-mix(in_oklab,var(--destructive)_10%,var(--background))] text-[color-mix(in_oklab,var(--destructive)_75%,var(--foreground))] border-[color-mix(in_oklab,var(--destructive)_25%,transparent)]",
  current:
    "border-foreground ring-foreground text-foreground font-semibold ring-1 ring-inset",
} as const;

/** A, B, C … the label the student and the teacher both refer to out loud. */
export function optionKey(index: number): string {
  return String.fromCharCode(65 + index);
}

/** Right, wrong, or nothing decided yet -- one word for a dot and a badge. */
export type Verdict = "correct" | "wrong" | "partial" | "pending" | "unanswered";
