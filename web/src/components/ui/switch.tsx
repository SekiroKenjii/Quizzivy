import type * as React from "react";
import { Switch as SwitchPrimitive } from "radix-ui";

import { cn } from "@/lib/utils";

/**
 * Switch is an on/off control. The default size is the deck's 38x22 with the
 * primary colour when on; `sm` (36x20) and `lg` (42x24, the student settings)
 * are the deck's other two sizes.
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
        "inline-flex shrink-0 items-center rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-50",
        "data-[state=checked]:bg-primary data-[state=unchecked]:bg-border h-5.5 w-9.5",
        size === "sm" && "h-5 w-9",
        size === "lg" && "h-6 w-10.5",
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        className={cn(
          "pointer-events-none block rounded-full transition-transform",
          "bg-switch-thumb shadow-thumb size-4.5 translate-x-0.5 data-[state=checked]:translate-x-4.5",
          size === "sm" &&
            "size-4 translate-x-0.5 data-[state=checked]:translate-x-4.5",
          size === "lg" && "size-5 translate-x-0.5 data-[state=checked]:translate-x-5",
        )}
      />
    </SwitchPrimitive.Root>
  );
}

export { Switch };
