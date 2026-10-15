import type { ColumnRect } from "@/features/roles/useColumnRect";

/**
 * ColumnHighlight is the 2px primary frame the matrix draws round one role's
 * column. It stays mounted while a rect is known, so moving to another column
 * slides it over 0.2s, and it moves at once under reduced motion. It only
 * draws: the header it frames is the control.
 */
export function ColumnHighlight({ rect }: Readonly<{ rect: ColumnRect | null }>) {
  if (rect === null) return null;
  return (
    <div
      aria-hidden="true"
      data-slot="column-highlight"
      className="bg-fg/5 border-primary pointer-events-none absolute inset-y-1 z-2 rounded-[0.625rem] border-2 transition-[left,width] duration-200 motion-reduce:transition-none"
      style={{ left: rect.left, width: rect.width }}
    />
  );
}
