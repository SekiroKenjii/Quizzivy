import type { ComponentProps, ElementType } from "react";
import { cn } from "@/lib/utils";

interface StickyAsideProps extends ComponentProps<"aside"> {
  /** The element to render: an aside by default, a nav for a page's navigation. */
  as?: ElementType;
  /**
   * Whether the column is docked beside the content now. A host that docks
   * the column by a container query leaves it out and adds
   * `@min-[…]/<container>:sticky-column` to `className` instead.
   */
  docked?: boolean;
}

/**
 * StickyAside is a page's side column, as the deck draws every one (V-02):
 * while docked beside the content it sticks 16px below the top of the page's
 * scroller and scrolls inside itself once it is taller than the window allows
 * (`sticky-column`); otherwise it is static. It always pads its content by
 * 1px above and 2px at the sides and below, and takes the same back as a
 * negative margin, so a focus ring or a card's shadow at its edge is not cut
 * off by the scrolling box and the content stays where it would be without it.
 * The host decides when the column is docked, by its own container's width:
 * with `docked`, or with a container query in `className`.
 */
export function StickyAside({
  as: Tag = "aside",
  docked = false,
  className,
  ...rest
}: Readonly<StickyAsideProps>) {
  return (
    <Tag
      data-sticky-aside=""
      className={cn(
        "-mx-0.5 -mt-px -mb-0.5 px-0.5 pt-px pb-0.5",
        docked && "sticky-column",
        className,
      )}
      {...rest}
    />
  );
}
