/**
 * PAPER_SURFACE keeps content that was drawn for white paper light in dark
 * mode (DG-35), from the paper tokens and with no theme branch: an image gets
 * the paper behind it, and a rich table is paper with its own ink, its rules
 * and header cells mixed from the two. The table rules are marked important
 * because content.css is unlayered. Put it on an ancestor of the content.
 */
export const PAPER_SURFACE = [
  "[&_img]:bg-paper",
  "[&_.content-table-scroll]:bg-paper [&_.content-table-scroll]:text-paper-fg [&_.content-table-scroll]:rounded-md",
  "[&_.content-table-scroll_:is(th,td)]:border-[color-mix(in_oklab,var(--paper-fg)_14%,var(--paper))]!",
  "[&_.content-table-scroll_th]:bg-[color-mix(in_oklab,var(--paper-fg)_5%,var(--paper))]!",
].join(" ");
