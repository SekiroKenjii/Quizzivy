import type * as React from "react";

import { cn } from "@/lib/utils";

/** The deck's `.sep`: a 1px rule in the border colour, no margin of its own. */
function Separator({ className, ...props }: React.ComponentProps<"hr">) {
  return (
    <hr
      data-slot="separator"
      className={cn("bg-border h-px border-0", className)}
      {...props}
    />
  );
}

export { Separator };
