/** Which page numbers to draw between Previous and Next. */
export type PageSlot = number | "gap";

export function pageWindow(page: number, pageCount: number): PageSlot[] {
  if (pageCount <= 1) return [];
  const keep = new Set<number>([1, pageCount]);
  for (let n = page - 1; n <= page + 1; n++) {
    if (n >= 1 && n <= pageCount) keep.add(n);
  }
  const slots: PageSlot[] = [];
  let previous = 0;
  for (const n of [...keep].sort((a, b) => a - b)) {
    if (n - previous === 2) slots.push(previous + 1);
    else if (n - previous > 2) slots.push("gap");
    slots.push(n);
    previous = n;
  }
  return slots;
}

export function pageCountOf(total: number, pageSize: number): number {
  return pageSize > 0 ? Math.ceil(total / pageSize) : 0;
}

/** A page number the URL can carry: an integer of at least 1, else 1. */
export function parsePage(raw: string | null): number {
  const n = Number(raw);
  return Number.isInteger(n) && n >= 1 ? n : 1;
}

/**
 * parsePageSize reads the rows per page a URL can carry: `raw` as a number
 * when it is one of `sizes`, else the first of `sizes`, the default. `sizes`
 * holds at least one size.
 */
export function parsePageSize(raw: string | null, sizes: readonly number[]): number {
  const n = Number(raw);
  return raw !== null && sizes.includes(n) ? n : sizes[0]!;
}

/**
 * PageRange is where a page sits in a list: the page clamped to the pages
 * there are, their count, never less than one, and the 1-based positions of
 * the page's first and last rows, both 0 for an empty list.
 */
export interface PageRange {
  page: number;
  pages: number;
  from: number;
  to: number;
}

/**
 * pageRange places `page` in a list of `total` rows at `pageSize` rows a page.
 * A page past the end is the last page. A size of zero or less puts every row
 * on one page.
 */
export function pageRange(page: number, pageSize: number, total: number): PageRange {
  const rows = Math.max(0, total);
  const size = pageSize > 0 ? pageSize : Math.max(1, rows);
  const pages = Math.max(1, Math.ceil(rows / size));
  const current = Math.min(Math.max(page, 1), pages);
  if (rows === 0) return { page: current, pages, from: 0, to: 0 };
  const from = (current - 1) * size + 1;
  return { page: current, pages, from, to: Math.min(rows, from + size - 1) };
}
