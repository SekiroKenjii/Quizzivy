import { Skeleton } from "@/components/ui/skeleton";

const ROWS = ["first", "second", "third"] as const;

/**
 * StudentSkeleton is the frame the splash fades onto while the student
 * console loads: the top bar and a page of grey blocks, as the Splash deck
 * draws it. The splash renders it outside the shell, so it sets its own deck
 * scale and paints its own background.
 */
export function StudentSkeleton() {
  return (
    <div data-scale="deck" className="bg-bg flex h-full flex-col">
      <div className="flex h-15 flex-none items-center gap-3 border-b px-5">
        <Skeleton className="h-5.5 w-27.5 rounded-sm" />
        <Skeleton className="hidden h-3 w-70 rounded-sm min-[768px]:block" />
        <span className="flex-1" />
        <Skeleton className="size-8" />
        <Skeleton className="size-8 rounded-full" />
      </div>
      <div className="mx-auto flex w-full max-w-240 flex-1 flex-col gap-4.5 p-7">
        <Skeleton className="h-6.5 w-60 rounded-sm" />
        <Skeleton className="h-3 w-80 max-w-full rounded-sm" />
        <div className="flex items-center gap-4 rounded-2xl border p-5">
          <div className="flex flex-1 flex-col gap-2.5">
            <Skeleton className="h-5 w-35 rounded-full" />
            <Skeleton className="h-4.5 w-[70%] rounded-sm" />
            <Skeleton className="h-3 w-1/2 rounded-sm" />
            <Skeleton className="h-1.5 w-[60%] rounded-sm" />
          </div>
          <Skeleton className="hidden h-11.5 w-37.5 rounded-lg min-[768px]:block" />
        </div>
        {ROWS.map((row) => (
          <div key={row} className="flex items-center gap-3.5 py-1.5">
            <Skeleton className="rounded-ctl size-11" />
            <div className="flex flex-1 flex-col gap-2">
              <Skeleton className="h-[0.8125rem] w-[55%] rounded-sm" />
              <Skeleton className="h-2.5 w-[35%] rounded-sm" />
            </div>
            <Skeleton className="h-5.5 w-20 rounded-full" />
          </div>
        ))}
      </div>
    </div>
  );
}
