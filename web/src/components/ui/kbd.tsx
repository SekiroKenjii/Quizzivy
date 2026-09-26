import type * as React from "react";

import { cn } from "@/lib/utils";

/** Kbd is a key cap, with the heavier bottom border that reads as one. */
export function Kbd({ className, ...props }: React.ComponentProps<"kbd">) {
  return (
    <kbd
      className={cn(
        "border-border bg-background text-muted-foreground inline-grid h-5 min-w-5 place-content-center rounded-sm border border-b-2 px-[0.3125rem] font-mono text-[0.6875rem]",
        "in-data-[scale=deck]:text-caption in-data-[scale=deck]:rounded-[4px] in-data-[scale=deck]:font-sans",
        className,
      )}
      {...props}
    />
  );
}
