import type * as React from "react";

import { cn } from "@/lib/utils";

/**
 * Matches the deck's `.checkbox` (kit.css): a 1rem square that fills with
 * --primary when checked and draws the tick as a clip-path, so there is no icon
 * to load and no wrapper element to align.
 *
 * A native input rather than a Radix primitive: it is only ever used inside a
 * label, where the native control already gives the click target, the focus
 * ring and the form semantics.
 */
function Checkbox({ className, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type="checkbox"
      data-slot="checkbox"
      className={cn(
        "border-input bg-background focus-visible:ring-ring checked:bg-primary checked:border-primary relative inline-grid size-4 flex-none cursor-pointer appearance-none place-content-center rounded-sm border transition-colors focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50",
        "checked:after:bg-primary-foreground checked:after:size-2.5 checked:after:content-['']",
        "checked:after:[clip-path:polygon(14%_47%,0_61%,39%_100%,100%_20%,85%_8%,38%_70%)]",
        className,
      )}
      {...props}
    />
  );
}

export { Checkbox };
