import type { ComponentType, ReactNode, SVGProps } from "react";

/**
 * Note is the deck's muted note: an icon on a round chip, a title line, and
 * the text under it, in a quiet box above the thing it explains. A note,
 * never a warning.
 */
export function Note({
  icon: Icon,
  title,
  children,
}: Readonly<{
  icon: ComponentType<SVGProps<SVGSVGElement>>;
  title: string;
  children: ReactNode;
}>) {
  return (
    <div role="note" className="bg-muted flex gap-3 rounded-xl px-4 py-3.5">
      <span className="bg-card grid size-7.5 flex-none place-items-center rounded-full">
        <Icon className="size-4" aria-hidden="true" />
      </span>
      <p className="text-ui min-w-0 leading-[1.55]">
        <span className="mb-0.5 block font-medium">{title}</span>
        {children}
      </p>
    </div>
  );
}
