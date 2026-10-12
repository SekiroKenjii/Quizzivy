import { useId, type ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * WorkspaceGrid lays an import's two panes side by side, `--pane-split`, from
 * 960px of the page's content width, and stacks them below it. While the
 * import is started, the status pane comes first in the stacked order.
 */
export function WorkspaceGrid({
  sources,
  status,
  statusFirst,
}: Readonly<{ sources: ReactNode; status: ReactNode; statusFirst: boolean }>) {
  return (
    <div className="@container/workspace min-w-0">
      <div className="grid items-start gap-4 @[960px]/workspace:grid-cols-(--pane-split)">
        {sources}
        <div
          className={cn(
            "min-w-0",
            statusFirst && "order-first @[960px]/workspace:order-none",
          )}
        >
          {status}
        </div>
      </div>
    </div>
  );
}

/**
 * Pane is one of the workspace's two cards: its heading with `aside` at the
 * end of the row, then `children`, which lay out their own padding.
 */
export function Pane({
  title,
  aside,
  children,
}: Readonly<{ title: string; aside?: ReactNode; children: ReactNode }>) {
  const headingId = useId();
  return (
    <section
      aria-labelledby={headingId}
      className="bg-card text-card-foreground shadow-card min-w-0 overflow-hidden rounded-xl border"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-3 border-b px-4.5 py-3.5">
        <h2 id={headingId} className="text-[15px] font-semibold">
          {title}
        </h2>
        {aside === undefined ? null : (
          <span className="text-muted-fg text-[12.5px]">{aside}</span>
        )}
      </div>
      {children}
    </section>
  );
}

/** PaneBody is a padded column of a pane's content; `ruled` draws a line above it. */
export function PaneBody({
  ruled = false,
  children,
}: Readonly<{ ruled?: boolean; children: ReactNode }>) {
  return (
    <div className={cn("flex flex-col gap-3.5 px-4.5 py-4", ruled && "border-t")}>
      {children}
    </div>
  );
}
