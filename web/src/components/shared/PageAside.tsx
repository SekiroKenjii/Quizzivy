import { useContext, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { SideColumn } from "@/components/shared/SideColumn";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import { PageAsideSlot, PageRailSlot } from "@/layouts/slots";

interface PageAsideProps {
  /** Names the landmark; a screen with more than one needs them distinct. */
  label: string;
  /** A right panel details the content; a left rail filters it (F-11). */
  side?: "right" | "left";
  /** S-08: below 1024px the navigator is a sheet, so the rail is not drawn. */
  hideBelow?: "lg";
  /** Where the same content goes below `hideBelow`, opened from the screen's own chrome. */
  sheet?: { open: boolean; onOpenChange: (open: boolean) => void };
  children: ReactNode;
}

/**
 * The one side column every screen goes through, at the two widths F-11 sets
 * and F-13 lets the teacher adjust.
 *
 * Rendered into the shell's slot (layouts/slots.ts) so it sits beside the
 * scrolling main rather than inside it. Without a slot it renders in place.
 */
export function PageAside({
  label,
  side = "right",
  hideBelow,
  sheet,
  children,
}: Readonly<PageAsideProps>) {
  const panelSlot = useContext(PageAsideSlot);
  const railSlot = useContext(PageRailSlot);
  const wide = useMediaQuery("(min-width: 1024px)");
  const slot = side === "left" ? railSlot : panelSlot;

  const aside = (
    <SideColumn
      column={side === "left" ? "rail" : "panel"}
      side={side}
      aria-label={label}
      className={cn(
        "space-y-5 overflow-y-auto",
        side === "left" ? "border-r p-4" : "border-l p-5",
        hideBelow === "lg" && "hidden lg:block",
      )}
    >
      {children}
    </SideColumn>
  );

  const column = slot === null ? aside : createPortal(aside, slot);
  if (sheet === undefined) return column;
  // One or the other, never both: the same ids cannot exist twice in the document.
  if (wide) return column;
  return (
    <Dialog open={sheet.open} onOpenChange={sheet.onOpenChange}>
      <DialogContent className="max-h-[80svh] space-y-5 overflow-y-auto">
        <DialogTitle className="sr-only">{label}</DialogTitle>
        {children}
      </DialogContent>
    </Dialog>
  );
}
