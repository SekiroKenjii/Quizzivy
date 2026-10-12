/**
 * FLAGGED is the one look of a flagged attempt wherever the teacher meets it
 * (D8): the Flagged KPI and stat, the roster's count and row, the Recent
 * activity marker and the flagged notification all read the danger tokens.
 */
export const FLAGGED = {
  tone: "danger",
  ink: "text-danger-ink",
  soft: "bg-danger-soft",
} as const;

/** TimelineMark is what a dot on the attempt timeline stands for. */
export type TimelineMark = "start" | "away" | "submitted" | "autosave" | "other";

/** TIMELINE_DOT is the deck's dot colour for each mark of the attempt timeline. */
export const TIMELINE_DOT: Readonly<Record<TimelineMark, string>> = {
  start: "bg-info",
  away: "bg-danger",
  submitted: "bg-success",
  autosave: "bg-border",
  other: "bg-muted-fg",
};

const AWAY_KINDS: ReadonlySet<string> = new Set([
  "tab_hidden",
  "window_blur",
  "fullscreen_exit",
  "auto_submit",
]);

/**
 * timelineMark names the dot of a recorded event: leaving the test and the
 * submission it forced are "away"; a lost connection, audio and every other
 * kind stay "other", so a network drop never reads as a focus loss.
 */
export function timelineMark(kind: string): TimelineMark {
  return AWAY_KINDS.has(kind) ? "away" : "other";
}

/** focusLossLabel is the roster's count of leaving the test: "—" for none or unknown, otherwise "{n}×". */
export function focusLossLabel(count: number | null | undefined): string {
  return count == null || count === 0 ? "—" : `${count}×`;
}
