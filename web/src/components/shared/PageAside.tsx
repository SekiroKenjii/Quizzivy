import { useContext, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";
import { PageAsideSlot, PageRailSlot } from "@/layouts/slots";

interface PageAsideProps {
  /** Names the landmark; a screen with more than one needs them distinct. */
  label: string;
  /**
   * Which edge of the content this column takes. A right panel details what
   * is selected in the middle (G-01, G-07, A-04, S-08); a left rail filters
   * what the middle shows (A-06). The two roles get different widths because
   * they hold different things -- a rail of checkboxes at panel width is
   * mostly empty -- and one width each, so neither drifts.
   */
  side?: "right" | "left";
  /** S-08: below 1024px the navigator is a sheet, so the rail is not drawn. */
  hideBelow?: "lg";
  children: ReactNode;
}

/**
 * The one side column every screen goes through.
 *
 * The deck drew the right panel four times at three widths and the left rail
 * at a fourth; here each role has one width, one padding and one rhythm, and
 * what goes inside is the screen's business. The deck's F-11 records the two
 * widths so it and the code answer "how wide is this?" the same way.
 *
 * It is a column beside the scrolling main, not part of it: rendered into the
 * shell's slot (see layouts/slots.ts) it runs the full height of the content
 * area, holds still while main scrolls, and scrolls by itself when its own
 * content outgrows the screen. Without a slot it renders in place, which is
 * right only inside a row the screen already lays out that way.
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
