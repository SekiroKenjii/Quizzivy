import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { cn } from "@/lib/utils";

const FOUR_COLUMNS_FROM = 640;

/**
 * StatStripItem is one figure of a StatStrip: its label, the value as the
 * screen formats it, an optional muted suffix drawn inside the value (" / 24",
 * "%") and the value's tone.
 */
export type StatStripItem = Readonly<{
  label: string;
  value: string;
  suffix?: string | undefined;
  tone?: "default" | "danger" | undefined;
}>;

/**
 * StatStripProps is what a StatStrip takes: its figures, in the order they
 * are drawn.
 */
export type StatStripProps = Readonly<{
  items: readonly StatStripItem[];
  className?: string | undefined;
}>;

/**
 * StatStrip is the deck's row of figures above a record's tabs: cells one
 * pixel apart on a border-coloured ground, four to a row when the console's
 * content area is at least 640px wide and two below that. It reads the
 * content width, not the viewport, so it changes when the sidebar collapses.
 * It is a description list: each label is a term and each value its
 * definition. It is drawn for four figures.
 */
export function StatStrip({ items, className }: StatStripProps) {
  const four = useContentWidthAtLeast(FOUR_COLUMNS_FROM);
  return (
    <dl
      className={cn(
        "bg-border grid gap-px overflow-hidden rounded-xl border",
        four ? "grid-cols-4" : "grid-cols-2",
        className,
      )}
    >
      {items.map((item) => (
        <div key={item.label} className="bg-card px-4 py-3.5">
          <dt className="text-muted-fg text-meta leading-normal">{item.label}</dt>
          <dd
            className={cn(
              "text-stat mt-0.5 leading-normal font-semibold tracking-[-0.02em] tabular-nums",
              item.tone === "danger" ? "text-danger-ink" : "text-fg",
            )}
          >
            {item.value}
            {item.suffix !== undefined && (
              <span className="text-muted-fg text-sm font-medium">{item.suffix}</span>
            )}
          </dd>
        </div>
      ))}
    </dl>
  );
}
