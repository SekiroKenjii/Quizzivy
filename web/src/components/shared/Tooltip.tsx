import type { ReactElement } from "react";
import { Tooltip as Primitive } from "radix-ui";
import { useDeckScale } from "@/components/ui/deck-scale";
import { cn } from "@/lib/utils";

/** Tooltip labels a compact control on hover and keyboard focus. */
export function Tooltip({
  label,
  children,
}: Readonly<{ label: string; children: ReactElement }>) {
  const deck = useDeckScale();
  return (
    <Primitive.Provider delayDuration={300}>
      <Primitive.Root>
        <Primitive.Trigger asChild>{children}</Primitive.Trigger>
        <Primitive.Portal>
          <Primitive.Content
            sideOffset={5}
            data-scale={deck ? "deck" : undefined}
            className={cn(
              "bg-foreground text-background z-50 rounded-md px-2 py-1 text-xs shadow-sm",
              deck && "data-[scale=deck]:z-(--z-popover)",
            )}
          >
            {label}
          </Primitive.Content>
        </Primitive.Portal>
      </Primitive.Root>
    </Primitive.Provider>
  );
}
