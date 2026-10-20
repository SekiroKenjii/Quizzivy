import type * as React from "react";

import { cn } from "@/lib/utils";

/**
 * Checkbox is a native checkbox drawn as a 1rem square: it fills with the
 * primary colour and shows a tick when checked, and a 2px bar when its
 * `indeterminate` property is set. The empty square is the deck's, a 4px
 * radius and the ring colour on the card fill. It keeps the
 * native control's click target, focus ring and form semantics.
 */
function Checkbox({ className, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type="checkbox"
      data-slot="checkbox"
      className={cn(
        "checked:bg-primary checked:border-primary relative inline-grid size-4 flex-none cursor-pointer appearance-none place-content-center border transition-colors disabled:cursor-not-allowed disabled:opacity-50",
        "checked:after:bg-primary-foreground checked:after:content-['']",
        "checked:after:[clip-path:polygon(14%_47%,0_61%,39%_100%,100%_20%,85%_8%,38%_70%)]",
        "indeterminate:bg-primary indeterminate:border-primary indeterminate:after:bg-primary-foreground indeterminate:after:h-0.5 indeterminate:after:w-2 indeterminate:after:content-['']",
        "border-ring bg-card rounded-[0.25rem] checked:after:size-[0.6875rem]",
        className,
      )}
      {...props}
    />
  );
}

export { Checkbox };
