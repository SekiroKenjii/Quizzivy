import type * as React from "react";
import { Switch as SwitchPrimitive } from "radix-ui";

import { cn } from "@/lib/utils";

// Matches the deck's `.switch` (kit.css): 2rem x 1.15rem, fully rounded, the
// track tinted from the foreground rather than the primary colour, because §12
// reserves colour for meaning.
function Switch({
  className,
  ...props
}: React.ComponentProps<typeof SwitchPrimitive.Root>) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        "focus-visible:ring-ring data-[state=checked]:bg-foreground data-[state=unchecked]:bg-foreground/22 inline-flex h-[1.15rem] w-8 shrink-0 items-center rounded-full transition-colors focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb className="bg-background pointer-events-none block size-[0.9rem] translate-x-[0.125rem] rounded-full shadow-sm transition-transform data-[state=checked]:translate-x-[0.975rem]" />
    </SwitchPrimitive.Root>
  );
}

export { Switch };
