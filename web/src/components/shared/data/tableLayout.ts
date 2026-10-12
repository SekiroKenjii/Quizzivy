import type { ReactNode } from "react";

/**
 * DataColumn is one column of a DataTable. `track` is its grid track, fixed
 * or fractional with a minimum (`120px`, `minmax(200px,2.2fr)`), never
 * content-sized: every row is a grid of its own, so `auto` would give each row
 * its own column width. `showFrom` is the content width from which the column
 * shows; without it the column always shows. `align: "end"` right-aligns the
 * header and the cell. `cell` draws the row's cell and is told which columns
 * are shown, so the first column can draw the table's sub-line for the ones
 * that are not. `aside` draws under `cell` in the same cell, outside the link
 * or button that opens the row: the place for a control in the first column.
 */
export interface DataColumn<T> {
  readonly id: string;
  readonly header: string;
  readonly track: string;
  readonly showFrom?: number;
  readonly align?: "end";
  readonly cell: (row: T, shown: ReadonlySet<string>) => ReactNode;
  readonly aside?: (row: T, shown: ReadonlySet<string>) => ReactNode;
}

/**
 * RowSize is a table's body row as the deck draws it: a fixed height, a
 * minimum height, or rows padded above and below by `padY`.
 */
export type RowSize =
  | { readonly height: number }
  | { readonly minHeight: number }
  | { readonly padY: number };

/** RowBox is the inline style a RowSize resolves to. */
export interface RowBox {
  readonly height?: number;
  readonly minHeight?: number;
  readonly paddingTop?: number;
  readonly paddingBottom?: number;
}

type ColumnLayout = Pick<DataColumn<never>, "id" | "track" | "showFrom">;

/** SELECT_TRACK is the checkbox column's track. */
export const SELECT_TRACK = "16px";

/** MENU_TRACK is the row menu's track unless the table gives its own. */
export const MENU_TRACK = "44px";

/** PAD_X is a row's horizontal padding in pixels unless the table gives its own. */
export const PAD_X = 16;

/** DENSE_CUT is what the "Compact tables" preference takes off a row's height, in pixels. */
export const DENSE_CUT = 16;

/**
 * thresholdsOf lists the distinct `showFrom` widths of `columns` in ascending
 * order, the form `useContentBand` takes.
 */
export function thresholdsOf(columns: readonly ColumnLayout[]): readonly number[] {
  const widths = new Set<number>();
  for (const column of columns) {
    if (column.showFrom !== undefined) widths.add(column.showFrom);
  }
  return [...widths].sort((a, b) => a - b);
}

/**
 * shownAt is the set of the ids of the columns visible in `band`, the number
 * of `thresholds` the content width meets: a column without `showFrom`, and
 * one whose `showFrom` is among the first `band` thresholds.
 */
export function shownAt(
  columns: readonly ColumnLayout[],
  thresholds: readonly number[],
  band: number,
): ReadonlySet<string> {
  const met = new Set(thresholds.slice(0, band));
  const shown = new Set<string>();
  for (const column of columns) {
    if (column.showFrom === undefined || met.has(column.showFrom)) shown.add(column.id);
  }
  return shown;
}

/**
 * templateOf joins the tracks of the columns in `shown`, in the order of
 * `columns`, into one `grid-template-columns` value, after the checkbox track
 * when the table selects and before `menuTrack` when it has a row menu.
 */
export function templateOf(
  columns: readonly ColumnLayout[],
  shown: ReadonlySet<string>,
  edges: Readonly<{ select: boolean; menuTrack?: string }>,
): string {
  const tracks = columns
    .filter((column) => shown.has(column.id))
    .map((column) => column.track);
  if (edges.select) tracks.unshift(SELECT_TRACK);
  if (edges.menuTrack !== undefined) tracks.push(edges.menuTrack);
  return tracks.join(" ");
}

/**
 * rowBox resolves a RowSize to the row's inline style. `dense` takes
 * DENSE_CUT off a fixed or a minimum height and halves a padded row's
 * vertical padding.
 */
export function rowBox(size: RowSize, dense: boolean): RowBox {
  if ("height" in size) {
    return { height: dense ? size.height - DENSE_CUT : size.height };
  }
  if ("minHeight" in size) {
    return { minHeight: dense ? size.minHeight - DENSE_CUT : size.minHeight };
  }
  const pad = dense ? size.padY / 2 : size.padY;
  return { paddingTop: pad, paddingBottom: pad };
}
