import type { MouseEvent } from "react";
import "./content.css";

/**
 * GapMark is the verdict a result's gap chip carries: the answer's mark, or
 * none when the review policy hides marks.
 */
export type GapMark = "correct" | "wrong" | null;

function chipState(open: boolean, word: string | null, mark: GapMark | undefined) {
  if (mark === "correct" || mark === "wrong") return mark;
  if (open) return "open";
  return word === null ? "empty" : "filled";
}

/**
 * GapChip is a gap of a cloze in its passage, as the deck draws it: the
 * question's number in a round mark, then the chosen word, cut with an
 * ellipsis past 160px. It is empty, filled once a word is chosen, or open
 * while its question is the one on screen. On a result (`mark` given) it is
 * the smaller chip in the verdict's colour, a wrong word struck through and
 * no word an em dash; a null mark leaves it neutral. It is a link when given
 * `href` and a button otherwise; `label` names it for a screen reader.
 */
export function GapChip({
  number,
  word,
  open = false,
  mark,
  label,
  href,
  onClick,
}: Readonly<{
  number: number;
  word: string | null;
  open?: boolean;
  mark?: GapMark | undefined;
  label: string;
  href?: string | undefined;
  onClick?: ((event: MouseEvent<HTMLElement>) => void) | undefined;
}>) {
  const state = chipState(open, word, mark);
  const empty = mark === undefined ? " " : "—";
  const face = (
    <>
      <span aria-hidden="true" className="content-chip-number">
        {number}
      </span>
      <span aria-hidden="true" className="content-chip-word">
        {word ?? empty}
      </span>
    </>
  );
  const size = mark === undefined ? undefined : "result";
  if (href !== undefined) {
    return (
      <a
        className="content-chip"
        data-state={state}
        data-size={size}
        data-word={word === null ? undefined : ""}
        href={href}
        aria-label={label}
        onClick={onClick}
      >
        {face}
      </a>
    );
  }
  return (
    <button
      type="button"
      className="content-chip"
      data-state={state}
      data-size={size}
      data-word={word === null ? undefined : ""}
      aria-label={label}
      aria-current={open ? "true" : undefined}
      onClick={onClick}
    >
      {face}
    </button>
  );
}
