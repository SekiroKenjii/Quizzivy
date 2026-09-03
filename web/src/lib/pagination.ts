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
