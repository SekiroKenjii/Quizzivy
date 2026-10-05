import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { cn } from "@/lib/utils";

const FOUR_COLUMNS_FROM = 640;

/** StatStripItem describes a formatted figure with an optional suffix and tone. */
export type StatStripItem = Readonly<{
  label: string;
  value: string;
  suffix?: string | undefined;
  tone?: "default" | "danger" | undefined;
}>;

/** StatStripProps supplies figures in their display order. */
export type StatStripProps = Readonly<{
  items: readonly StatStripItem[];
  className?: string | undefined;
}>;

/** StatStrip lays out figures in two or four columns according to the registered content width. */
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
