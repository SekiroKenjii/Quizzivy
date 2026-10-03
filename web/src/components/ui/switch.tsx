import type * as React from "react";
import { Switch as SwitchPrimitive } from "radix-ui";

import { cn } from "@/lib/utils";

/**
 * Switch is an on/off control. On a `data-scale="deck"` surface the default
 * size is the deck's 38x22 with the primary colour when on; elsewhere it
 * renders today's switch. `sm` (36x20) and `lg` (42x24, the student
 * settings) are the deck's other two sizes and have that geometry everywhere.
 */
function Switch({
  className,
  size = "md",
  ...props
}: React.ComponentProps<typeof SwitchPrimitive.Root> & { size?: "sm" | "md" | "lg" }) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      data-size={size}
      className={cn(
        "data-[state=checked]:bg-foreground data-[state=unchecked]:bg-foreground/22 inline-flex h-[1.15rem] w-8 shrink-0 items-center rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-50",
        "in-data-[scale=deck]:data-[state=checked]:bg-primary in-data-[scale=deck]:data-[state=unchecked]:bg-border in-data-[scale=deck]:h-5.5 in-data-[scale=deck]:w-9.5",
        size === "sm" && "h-5 w-9 in-data-[scale=deck]:h-5 in-data-[scale=deck]:w-9",
        size === "lg" &&
          "h-6 w-10.5 in-data-[scale=deck]:h-6 in-data-[scale=deck]:w-10.5",
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        className={cn(
          "bg-background pointer-events-none block size-[0.9rem] translate-x-[0.125rem] rounded-full shadow-sm transition-transform data-[state=checked]:translate-x-[0.975rem]",
          "in-data-[scale=deck]:bg-switch-thumb in-data-[scale=deck]:size-4.5 in-data-[scale=deck]:translate-x-0.5 in-data-[scale=deck]:data-[state=checked]:translate-x-4.5",
          size === "sm" &&
            "size-4 translate-x-0.5 in-data-[scale=deck]:size-4 data-[state=checked]:translate-x-4.5 in-data-[scale=deck]:data-[state=checked]:translate-x-4.5",
          size === "lg" &&
            "size-5 translate-x-0.5 in-data-[scale=deck]:size-5 data-[state=checked]:translate-x-5 in-data-[scale=deck]:data-[state=checked]:translate-x-5",
        )}
      />
    </SwitchPrimitive.Root>
  );
}

export { Switch };
