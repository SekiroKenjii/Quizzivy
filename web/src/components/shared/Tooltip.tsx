import type { ReactElement } from "react";
import { Tooltip as Primitive } from "radix-ui";

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
            className="bg-foreground text-background z-50 rounded-md px-2 py-1 text-xs shadow-sm"
          >
            {label}
          </Primitive.Content>
        </Primitive.Portal>
      </Primitive.Root>
    </Primitive.Provider>
  );
}
