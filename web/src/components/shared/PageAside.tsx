import { useContext, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";
import { PageAsideSlot } from "@/layouts/slots";

interface PageAsideProps {
  /** Names the landmark; a screen with more than one needs them distinct. */
  label: string;
  /** S-08: below 1024px the navigator is a sheet, so the rail is not drawn. */
  hideBelow?: "lg";
  children: ReactNode;
}

/**
 * The one side panel every screen goes through.
 *
 * The deck draws it four times at three widths (G-01 and A-04 at 20rem, G-07
 * at 24rem, S-08 at 18rem) and with two paddings; here it is one width, one
 * padding, one rhythm, and what goes inside is the screen's business.
 *
 * It is a column beside the scrolling main, not part of it: rendered into the
 * shell's slot (see layouts/slots.ts) it runs the full height of the content
 * area, holds still while main scrolls, and scrolls by itself when its own
 * content outgrows the screen. Without a slot it renders in place, which is
 * right only inside a row the screen already lays out that way.
 */
export function PageAside({ label, hideBelow, children }: PageAsideProps) {
  const slot = useContext(PageAsideSlot);
  const aside = (
    <aside
      aria-label={label}
      className={cn(
        "w-80 shrink-0 space-y-5 overflow-y-auto border-l p-5",
        hideBelow === "lg" && "hidden lg:block",
      )}
    >
      {children}
    </aside>
  );
  return slot === null ? aside : createPortal(aside, slot);
}
