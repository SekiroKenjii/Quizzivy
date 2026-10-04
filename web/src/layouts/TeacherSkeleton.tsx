import { Skeleton } from "@/components/ui/skeleton";
import { readSidebarState } from "@/layouts/shell/sidebarState";
import { cn } from "@/lib/utils";

const NAV_ROWS = [
  "w-20",
  "w-25.75",
  "w-31.5",
  "w-22.25",
  "w-28",
  "w-33.75",
  "w-24.5",
  "w-30.25",
] as const;
const CARDS = ["first", "second", "third", "fourth"] as const;
const ROWS = ["first", "second", "third", "fourth", "fifth"] as const;

/**
 * TeacherSkeleton is the frame the splash fades onto while the teacher
 * console loads: the sidebar, the top bar and a page of grey blocks, as the
 * Splash deck draws it. The sidebar column shows from 768px, at 60px with
 * its tile and icon blocks alone when the stored sidebar state is collapsed.
 * The splash renders it outside the shell, so it sets its own deck scale and
 * paints its own background.
 */
export function TeacherSkeleton() {
  const collapsed = readSidebarState() === "collapsed";
  return (
    <div data-scale="deck" className="bg-bg flex h-full">
      <div
        data-slot="skeleton-sidebar"
        className={cn(
          "bg-sidebar hidden flex-none flex-col gap-2.5 border-r px-3 py-3.5 min-[768px]:flex",
          collapsed ? "w-15 items-center" : "w-62",
        )}
      >
        <div className="mb-2.5 flex items-center gap-2.5">
          <Skeleton className="size-8" />
          {!collapsed && <Skeleton className="h-3 w-27.5 rounded-sm" />}
        </div>
        {NAV_ROWS.map((width) => (
          <div key={width} className="flex h-7.5 items-center gap-2.5 px-2">
            <Skeleton className="size-4 rounded-[0.25rem]" />
            {!collapsed && <Skeleton className={cn("h-2.5 rounded-sm", width)} />}
          </div>
        ))}
      </div>
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex h-14 flex-none items-center gap-3 border-b px-5">
          <Skeleton className="size-6 rounded-sm" />
          <Skeleton className="hidden h-3 w-45 rounded-sm min-[768px]:block" />
          <span className="flex-1" />
          <Skeleton className="size-8" />
          <Skeleton className="size-8 rounded-full" />
        </div>
        <div className="mx-auto flex w-full max-w-330 flex-1 flex-col gap-4.5 p-7">
          <Skeleton className="h-6.5 w-50 rounded-sm" />
          <Skeleton className="h-3 w-70 max-w-full rounded-sm" />
          <div className="grid grid-cols-2 gap-3 min-[768px]:grid-cols-4">
            {CARDS.map((card) => (
              <div
                key={card}
                className="flex min-h-24 flex-col gap-2.5 rounded-xl border p-4"
              >
                <Skeleton className="h-3 w-2/5 rounded-sm" />
                <Skeleton className="h-6 w-3/5 rounded-sm" />
                <Skeleton className="h-2.5 w-[30%] rounded-sm" />
              </div>
            ))}
          </div>
          <div className="overflow-hidden rounded-xl border">
            <div className="bg-muted h-10" />
            {ROWS.map((row) => (
              <div key={row} className="flex items-center gap-3.5 border-t px-4 py-3.5">
                <Skeleton className="size-7 rounded-full" />
                <Skeleton className="h-3 w-[30%] rounded-sm" />
                <span className="flex-1" />
                <Skeleton className="h-5 w-17.5 rounded-full" />
                <Skeleton className="hidden h-3 w-15 rounded-sm min-[768px]:block" />
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
