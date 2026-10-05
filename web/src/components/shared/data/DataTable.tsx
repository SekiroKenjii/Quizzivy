import { memo, useId, useMemo, type MouseEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import {
  BulkSelectAll,
  BulkSelectRow,
  type BulkSelection,
} from "@/components/shared/BulkSelection";
import { RowMenu } from "@/components/shared/RowMenu";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useContentBand } from "@/layouts/shell/contentWidth";
import { cn } from "@/lib/utils";
import {
  MENU_TRACK,
  PAD_X,
  rowBox,
  shownAt,
  templateOf,
  thresholdsOf,
  type DataColumn,
  type RowBox,
  type RowSize,
} from "./tableLayout";

export type { DataColumn, RowSize } from "./tableLayout";

type Item = { id: string };

type Opening<T> =
  | {
      readonly rowHref: ((row: T) => string) | undefined;
      readonly onOpen?: undefined;
    }
  | {
      readonly onOpen: ((row: T) => void) | undefined;
      readonly rowHref?: undefined;
    }
  | { readonly rowHref?: undefined; readonly onOpen?: undefined };

type Selecting<T extends Item> =
  | {
      readonly selection: BulkSelection<T> | undefined;
      readonly rowName: (row: T) => string;
    }
  | { readonly selection?: undefined; readonly rowName?: undefined };

type Select<T extends Item> = Readonly<{
  selection: BulkSelection<T>;
  name: (row: T) => string;
}>;

/** DataTableProps supplies columns, rows and optional selection, opening, per-row tones and menus while preserving plain default rows. */
export type DataTableProps<T extends Item> = Opening<T> &
  Selecting<T> & {
    readonly label: string;
    readonly columns: readonly DataColumn<T>[];
    readonly rows: readonly T[];
    readonly rowSize: RowSize;
    readonly dense?: boolean | undefined;
    readonly rowTone?: ((row: T) => "danger" | "selected" | undefined) | undefined;
    readonly canOpen?: ((row: T) => boolean) | undefined;
    readonly shown?: ReadonlySet<string> | undefined;
    readonly menu?: ((row: T) => ReactNode) | undefined;
    readonly menuTrack?: string | undefined;
    readonly padX?: number | undefined;
    readonly card?: ((row: T) => ReactNode) | undefined;
    readonly cardLayout?: "stacked" | "joined" | undefined;
    readonly framed?: boolean | undefined;
    readonly empty?: ReactNode;
    readonly footer?: ReactNode;
  };

const NO_THRESHOLDS: readonly number[] = [];

const OPENER = "data-table-open";

const CONTROLS =
  "a, button, input, select, textarea, label, summary, [role='button'], [role='link'], [role='checkbox'], [role='switch'], [role='combobox'], [tabindex]:not([tabindex='-1']), [contenteditable='true']";

const FRAME = "bg-card shadow-card rounded-xl border";

const EMPTY_LINE = "text-muted-fg text-ui p-10 text-center leading-normal";

const STACKED_CARD =
  "bg-card shadow-card text-ui flex w-full flex-col gap-2.5 rounded-xl border p-3.5 text-left leading-[normal]";

const JOINED_ROW =
  "flex w-full items-center gap-3 px-3.5 py-3 text-left text-base leading-normal -outline-offset-2!";

function drawsNothing(node: ReactNode) {
  return node == null || typeof node === "boolean" || node === "";
}

function openFromRow(event: MouseEvent<HTMLElement>) {
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
  const row = event.currentTarget;
  const target = event.target;
  if (!(target instanceof Element) || !row.contains(target)) return;
  const control = target.closest(CONTROLS);
  if (control !== null && row.contains(control)) return;
  row.querySelector<HTMLElement>(`[data-slot="${OPENER}"]`)?.click();
}

function Opener<T>({
  row,
  rowHref,
  onOpen,
  className,
  children,
}: Readonly<{
  row: T;
  rowHref: ((row: T) => string) | undefined;
  onOpen: ((row: T) => void) | undefined;
  className: string;
  children: ReactNode;
}>) {
  if (rowHref !== undefined) {
    return (
      <Link data-slot={OPENER} to={rowHref(row)} className={className}>
        {children}
      </Link>
    );
  }
  if (onOpen !== undefined) {
    return (
      <button
        type="button"
        data-slot={OPENER}
        className={cn("cursor-pointer", className)}
        onClick={() => onOpen(row)}
      >
        {children}
      </button>
    );
  }
  return <div className={className}>{children}</div>;
}

function CellContent<T>({
  column,
  row,
  shown,
}: Readonly<{ column: DataColumn<T>; row: T; shown: ReadonlySet<string> }>) {
  return column.cell(row, shown);
}

const Cell = memo(CellContent) as typeof CellContent;

function AsideContent<T>({
  column,
  row,
  shown,
}: Readonly<{ column: DataColumn<T>; row: T; shown: ReadonlySet<string> }>) {
  return column.aside?.(row, shown);
}

const Aside = memo(AsideContent) as typeof AsideContent;

function HeaderRow<T extends Item>({
  visible,
  rows,
  select,
  hasMenu,
  template,
  padX,
}: Readonly<{
  visible: readonly DataColumn<T>[];
  rows: readonly T[];
  select: Select<T> | undefined;
  hasMenu: boolean;
  template: string;
  padX: number;
}>) {
  const { t } = useTranslation();
  return (
    <div
      role="row"
      className="bg-muted text-muted-fg text-meta grid h-10 items-center gap-3 leading-normal font-medium"
      style={{ gridTemplateColumns: template, paddingLeft: padX, paddingRight: padX }}
    >
      {select === undefined ? null : (
        <div role="columnheader" className="flex">
          <BulkSelectAll items={rows} selection={select.selection} />
        </div>
      )}
      {visible.map((column) => (
        <div
          key={column.id}
          role="columnheader"
          className={cn("min-w-0", column.align === "end" && "text-right")}
        >
          {column.header}
        </div>
      ))}
      {hasMenu ? (
        <div role="columnheader" className="relative">
          <span className="sr-only">{t("common.actions")}</span>
        </div>
      ) : null}
    </div>
  );
}

function BodyRow<T extends Item>({
  row,
  visible,
  shown,
  labelId,
  rowHref,
  onOpen,
  select,
  menu,
  template,
  padX,
  box,
  tone,
  canOpen,
}: Readonly<{
  row: T;
  visible: readonly DataColumn<T>[];
  shown: ReadonlySet<string>;
  labelId: string;
  rowHref: ((row: T) => string) | undefined;
  onOpen: ((row: T) => void) | undefined;
  select: Select<T> | undefined;
  menu: ((row: T) => ReactNode) | undefined;
  template: string;
  padX: number;
  box: RowBox;
  tone: "danger" | "selected" | undefined;
  canOpen: boolean;
}>) {
  const opens = canOpen && (rowHref !== undefined || onOpen !== undefined);
  const menuItems = menu?.(row);
  const emptyMenu =
    menuItems == null ||
    menuItems === false ||
    (Array.isArray(menuItems) && menuItems.length === 0);
  return (
    // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/interactive-supports-focus -- the click is a pointer shortcut to the link or button in the row's first cell, which is what the keyboard reaches
    <div
      role="row"
      aria-labelledby={labelId}
      aria-selected={select?.selection.selected.has(row.id)}
      className={cn(
        "text-ui hover:bg-muted grid items-center gap-3 border-t leading-normal",
        opens && "cursor-pointer",
        tone === "danger" && "bg-danger-soft",
        tone === "selected" && "bg-muted",
      )}
      style={{
        gridTemplateColumns: template,
        paddingLeft: padX,
        paddingRight: padX,
        ...box,
      }}
      onClick={opens ? openFromRow : undefined}
    >
      {select === undefined ? null : (
        <div role="cell" className="flex">
          <BulkSelectRow
            item={row}
            name={select.name(row)}
            selection={select.selection}
          />
        </div>
      )}
      {visible.map((column, index) => (
        <div
          key={column.id}
          role="cell"
          id={index === 0 ? labelId : undefined}
          className={cn("min-w-0", column.align === "end" && "text-right")}
        >
          {index === 0 && opens ? (
            <Opener
              row={row}
              rowHref={rowHref}
              onOpen={onOpen}
              className="block w-full min-w-0 rounded-sm text-left"
            >
              <Cell column={column} row={row} shown={shown} />
            </Opener>
          ) : (
            <Cell column={column} row={row} shown={shown} />
          )}
          {column.aside === undefined ? null : (
            <Aside column={column} row={row} shown={shown} />
          )}
        </div>
      ))}
      {menu === undefined ? null : (
        <div role="cell" className="flex">
          {emptyMenu ? null : <RowMenu>{menuItems}</RowMenu>}
        </div>
      )}
    </div>
  );
}

function CardList<T extends Item>({
  label,
  rows,
  card,
  joined,
  canOpen,
  rowHref,
  onOpen,
  empty,
  footer,
}: Readonly<{
  label: string;
  rows: readonly T[];
  card: (row: T) => ReactNode;
  joined: boolean;
  canOpen: ((row: T) => boolean) | undefined;
  rowHref: ((row: T) => string) | undefined;
  onOpen: ((row: T) => void) | undefined;
  empty: ReactNode;
  footer: ReactNode;
}>) {
  const list =
    rows.length === 0 ? null : (
      <ul aria-label={label} className={joined ? undefined : "flex flex-col gap-2.5"}>
        {rows.map((row) => (
          <li key={row.id} className={joined ? "border-t first:border-t-0" : undefined}>
            <Opener
              row={row}
              rowHref={canOpen?.(row) === false ? undefined : rowHref}
              onOpen={canOpen?.(row) === false ? undefined : onOpen}
              className={joined ? JOINED_ROW : STACKED_CARD}
            >
              {card(row)}
            </Opener>
          </li>
        ))}
      </ul>
    );
  const none =
    rows.length > 0 || drawsNothing(empty) ? null : (
      <div className={EMPTY_LINE}>{empty}</div>
    );
  if (joined) {
    return (
      <div data-slot="data-table" className="bg-card overflow-hidden rounded-xl border">
        {list}
        {none}
        {footer}
      </div>
    );
  }
  return (
    <div data-slot="data-table" className="flex flex-col gap-2.5">
      {list}
      {none === null ? null : <div className={FRAME}>{none}</div>}
      {footer}
    </div>
  );
}

/**
 * DataTable is the console's list table as the design deck draws it: a card
 * holding a grid of rows with the ARIA table roles, one
 * `grid-template-columns` shared by the header and every row. A column shows
 * from its `showFrom` content width, or by the `shown` set when the screen
 * gives one. Every cell is told the visible set, which keeps its identity
 * until it changes, and a cell renders again only when its row, its column or
 * that set does: whatever else a cell draws from belongs in a component of
 * its own. A row that opens holds a real link or button around what its first
 * column's `cell` draws, which is what the keyboard reaches, so that `cell`
 * holds no control: one goes in the column's `aside`. The first cell, aside
 * included, names the row. A click elsewhere on the row presses the link or
 * button, unless a modifier key is held or the click landed on another
 * control in the row or outside the row, as one in an open menu does. The row
 * itself is never focusable. Below 768px a table that has `card` renders a
 * list of cards instead, without checkboxes, menus or asides. Content wider
 * than the table scrolls sideways inside it. Loading and failure stay with
 * the screen.
 */
export function DataTable<T extends Item>(props: DataTableProps<T>) {
  const {
    label,
    columns,
    rows,
    rowSize,
    dense = false,
    rowTone,
    canOpen,
    shown: given,
    rowHref,
    onOpen,
    menu,
    menuTrack = MENU_TRACK,
    padX = PAD_X,
    card,
    cardLayout = "stacked",
    framed = true,
    empty,
    footer,
  } = props;
  const select =
    props.selection === undefined
      ? undefined
      : { selection: props.selection, name: props.rowName };
  const baseId = useId();
  const wide = useMediaQuery("(min-width: 768px)");
  const own = given === undefined;
  const thresholds = useMemo(
    () => (own ? thresholdsOf(columns) : NO_THRESHOLDS),
    [columns, own],
  );
  const band = useContentBand(thresholds);
  const derived = useMemo(
    () => shownAt(columns, thresholds, band),
    [columns, thresholds, band],
  );
  const shown = given ?? derived;

  if (card !== undefined && !wide) {
    return (
      <CardList
        label={label}
        rows={rows}
        card={card}
        joined={cardLayout === "joined"}
        canOpen={canOpen}
        rowHref={rowHref}
        onOpen={onOpen}
        empty={empty}
        footer={footer}
      />
    );
  }

  const visible = columns.filter((column) => shown.has(column.id));
  const template = templateOf(columns, shown, {
    select: select !== undefined,
    ...(menu === undefined ? {} : { menuTrack }),
  });
  const box = rowBox(rowSize, dense);
  return (
    <div
      data-slot="data-table"
      className={cn("overflow-x-auto overflow-y-hidden", framed && FRAME)}
    >
      <div role="table" aria-label={label} className="min-w-min">
        <div role="rowgroup">
          <HeaderRow
            visible={visible}
            rows={rows}
            select={select}
            hasMenu={menu !== undefined}
            template={template}
            padX={padX}
          />
        </div>
        {rows.length === 0 ? null : (
          <div role="rowgroup">
            {rows.map((row) => (
              <BodyRow
                key={row.id}
                row={row}
                visible={visible}
                shown={shown}
                labelId={`${baseId}-${row.id}`}
                rowHref={rowHref}
                onOpen={onOpen}
                select={select}
                menu={menu}
                template={template}
                padX={padX}
                box={box}
                tone={rowTone?.(row)}
                canOpen={canOpen?.(row) ?? true}
              />
            ))}
          </div>
        )}
      </div>
      {rows.length > 0 || drawsNothing(empty) ? null : (
        <div className={cn(EMPTY_LINE, "border-t")}>{empty}</div>
      )}
      {footer}
    </div>
  );
}
