import type { ComponentType, ReactNode, SVGProps } from "react";

/**
 * The deck's muted alert (S-05): one icon, one short sentence, in a quiet box
 * above or below the thing it explains. A note, never a warning.
 */
export function Note({
  icon: Icon,
  children,
}: Readonly<{
  icon: ComponentType<SVGProps<SVGSVGElement>>;
  children: ReactNode;
}>) {
  return (
    <div
      role="note"
      className="bg-muted/30 flex items-start gap-2.5 rounded-md border px-3 py-2.5"
    >
      <Icon
        className="text-muted-foreground mt-0.5 size-4 shrink-0"
        aria-hidden="true"
      />
      <p className="text-xs leading-relaxed">{children}</p>
    </div>
  );
}
