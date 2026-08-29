import type * as React from "react";

import { cn } from "@/lib/utils";

/** The deck's `.kbd`: a key cap, with the heavier bottom border that reads as one. */
export function Kbd({ className, ...props }: React.ComponentProps<"kbd">) {
  return (
    <kbd
      className={cn(
        "border-border bg-background text-muted-foreground inline-grid h-5 min-w-5 place-content-center rounded-sm border border-b-2 px-[0.3125rem] font-mono text-[0.6875rem]",
        className,
      )}
      {...props}
    />
  );
}
