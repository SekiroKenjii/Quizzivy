import type { MouseEvent } from "react";
import "./content.css";

function chipState(open: boolean, word: string | null): "open" | "empty" | "filled" {
  if (open) return "open";
  return word === null ? "empty" : "filled";
}

/**
 * GapChip is a gap of a cloze in its passage, as the deck draws it: the
 * question's number in a round mark, then the chosen word, cut with an
 * ellipsis past 160px. It is empty, filled once a word is chosen, or open
 * while its question is the one on screen. It is a link when given `href`
 * and a button otherwise; `label` names it for a screen reader.
 */
export function GapChip({
  number,
  word,
  open = false,
  label,
  href,
  onClick,
}: Readonly<{
  number: number;
  word: string | null;
  open?: boolean;
  label: string;
  href?: string | undefined;
  onClick?: ((event: MouseEvent<HTMLElement>) => void) | undefined;
}>) {
  const state = chipState(open, word);
  const face = (
    <>
      <span aria-hidden="true" className="content-chip-number">
        {number}
      </span>
      <span aria-hidden="true" className="content-chip-word">
        {word ?? " "}
      </span>
    </>
  );
  if (href !== undefined) {
    return (
      <a
        className="content-chip"
        data-state={state}
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
      aria-label={label}
      aria-current={open ? "true" : undefined}
      onClick={onClick}
    >
      {face}
    </button>
  );
}
