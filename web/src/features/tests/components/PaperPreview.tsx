import { lazy, Suspense, type ComponentProps } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import type { EnginePreview as Engine } from "@/features/take-test";
import { PreviewFrameMessage } from "./StudentPreviewPane";

const EnginePreview = lazy(() =>
  import("@/features/take-test").then((engine) => ({ default: engine.EnginePreview })),
);

/**
 * PaperPreview is the take-test engine's preview in a frame, loaded as its
 * own chunk the first time a preview opens; until it arrives the frame shows
 * the skeleton of a paper.
 */
export function PaperPreview(props: Readonly<ComponentProps<typeof Engine>>) {
  return (
    <Suspense fallback={<PaperSkeleton />}>
      <EnginePreview {...props} />
    </Suspense>
  );
}

/** PaperSkeleton is the frame's loading state: a paper's shape, busy. */
export function PaperSkeleton() {
  return (
    <PreviewFrameMessage>
      <div aria-busy="true" className="flex flex-col gap-3">
        <Skeleton className="h-5 w-1/2" />
        <Skeleton className="h-32 w-full rounded-xl" />
        <Skeleton className="h-32 w-full rounded-xl" />
      </div>
    </PreviewFrameMessage>
  );
}
