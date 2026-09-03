import { useContext, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";
import { PageAsideSlot, PageRailSlot } from "@/layouts/slots";

interface PageAsideProps {
  /** Names the landmark; a screen with more than one needs them distinct. */
  label: string;
  /** A right panel details the content; a left rail filters it (F-11). */
  side?: "right" | "left";
  /** S-08: below 1024px the navigator is a sheet, so the rail is not drawn. */
  hideBelow?: "lg";
  children: ReactNode;
}

/**
 * The one side column every screen goes through, at the two widths F-11 sets.
 *
 * Rendered into the shell's slot (layouts/slots.ts) so it sits beside the
 * scrolling main rather than inside it. Without a slot it renders in place.
 */
export function PageAside({
  label,
  side = "right",
  hideBelow,
  children,
}: PageAsideProps) {
  const panelSlot = useContext(PageAsideSlot);
  const railSlot = useContext(PageRailSlot);
  const slot = side === "left" ? railSlot : panelSlot;

  const aside = (
    <aside
      aria-label={label}
      className={cn(
        "shrink-0 space-y-5 overflow-y-auto",
        side === "left" ? "w-56 border-r p-4" : "w-80 border-l p-5",
        hideBelow === "lg" && "hidden lg:block",
      )}
    >
      {children}
    </aside>
  );
  return slot === null ? aside : createPortal(aside, slot);
}
