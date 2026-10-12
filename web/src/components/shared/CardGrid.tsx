/* eslint-disable jsx-a11y/no-redundant-roles -- Preserve explicit list semantics with marker-free styling. */
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/** CardGridProps supplies stable keys, track width and caller-owned card content. */
export type CardGridProps<T> = Readonly<{
  label: string;
  items: readonly T[];
  itemKey: (item: T) => string;
  min: number;
  gap?: 10 | 12;
  className?: string;
  children: (item: T) => ReactNode;
}>;

/** CardGrid lays out a named list with auto-fill tracks capped to the available width. */
export function CardGrid<T>({
  label,
  items,
  itemKey,
  min,
  gap = 12,
  className,
  children,
}: CardGridProps<T>) {
  if (items.length === 0) return null;
  return (
    <ul
      role="list"
      aria-label={label}
      className={cn("m-0 grid min-w-0 list-none p-0", className)}
      style={{
        gridTemplateColumns: `repeat(auto-fill, minmax(min(${min}px, 100%), 1fr))`,
        gap,
      }}
    >
      {items.map((item) => (
        <li key={itemKey(item)} className="grid min-w-0">
          {children(item)}
        </li>
      ))}
    </ul>
  );
}
