import type { ReactElement } from "react";
import { Tooltip as Primitive } from "radix-ui";
import { cn } from "@/lib/utils";

/** Tooltip labels a compact control on hover and keyboard focus. */
export function Tooltip({
  label,
  children,
}: Readonly<{ label: string; children: ReactElement }>) {
  return (
    <Primitive.Provider delayDuration={300}>
      <Primitive.Root>
        <Primitive.Trigger asChild>{children}</Primitive.Trigger>
        <Primitive.Portal>
          <Primitive.Content
            sideOffset={5}
            className={cn(
              "bg-foreground text-background rounded-md px-2 py-1 text-xs shadow-sm",
              "z-(--z-popover)",
            )}
          >
            {label}
          </Primitive.Content>
        </Primitive.Portal>
      </Primitive.Root>
    </Primitive.Provider>
  );
}
