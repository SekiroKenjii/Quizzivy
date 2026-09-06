import { useRef, type ComponentProps, type ElementType, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { MIDDLE_MIN, useColumnWidth, type ColumnRole } from "@/hooks/useColumnWidth";
import { cn } from "@/lib/utils";

const STEP = 16;

interface SideColumnProps extends Omit<ComponentProps<"aside">, "children"> {
  /** Which remembered width this column takes (F-13). */
  column: ColumnRole;
  /** Which side of the middle column it sits on; the handle goes on the other edge. */
  side: "left" | "right";
  /** The element to render; a nav for the shell's sidebar, an aside elsewhere. */
  as?: ElementType;
  children: ReactNode;
}

/**
 * F-13: a side column whose inner edge is a handle. Drag it, or focus it and
 * use the arrow keys; double-click puts the deck's width back. The middle
 * column (`data-resize-middle` under the nearest `data-columns`) keeps
 * F-12's minimum whatever the sides are dragged to.
 */
export function SideColumn({
  column: role,
  side,
  as: Tag = "aside",
  className,
  style,
  children,
  ...rest
}: Readonly<SideColumnProps>) {
  const { t } = useTranslation();
  const { width, setWidth, reset, limits } = useColumnWidth(role);
  const column = useRef<HTMLElement | null>(null);
  const drag = useRef<{ x: number; width: number } | null>(null);

  /** How wide this column may grow before the middle falls under its floor. */
  const ceiling = () => {
    const el = column.current;
    const middle = el
      ?.closest("[data-columns]")
      ?.querySelector<HTMLElement>(":scope > [data-resize-middle]");
    // No layout (jsdom, display:none) reports zero; the static limit is all we have then.
    if (!el || !middle || middle.offsetWidth === 0) return limits.max;
    return Math.min(limits.max, el.offsetWidth + middle.offsetWidth - MIDDLE_MIN);
  };
  const apply = (next: number) => setWidth(Math.min(next, ceiling()));

  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) return;
    drag.current = { x: event.clientX, width };
    event.currentTarget.setPointerCapture(event.pointerId);
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
  };
  const onPointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    if (drag.current === null) return;
    const dx = event.clientX - drag.current.x;
    apply(drag.current.width + (side === "left" ? dx : -dx));
  };
  const onPointerUp = (event: React.PointerEvent<HTMLDivElement>) => {
    if (drag.current === null) return;
    drag.current = null;
    event.currentTarget.releasePointerCapture(event.pointerId);
    document.body.style.cursor = "";
    document.body.style.userSelect = "";
  };
  const onKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    const grow = side === "left" ? "ArrowRight" : "ArrowLeft";
    const shrink = side === "left" ? "ArrowLeft" : "ArrowRight";
    if (event.key === grow) apply(width + STEP);
    else if (event.key === shrink) apply(width - STEP);
    else if (event.key === "Home") apply(limits.min);
    else if (event.key === "End") apply(limits.max);
    else return;
    event.preventDefault();
  };

  return (
    <Tag
      ref={column}
      data-side-column={role}
      className={cn("relative shrink-0", className)}
      style={{ ...style, width }}
      {...rest}
    >
      {children}
      {/* eslint-disable jsx-a11y/no-noninteractive-element-interactions, jsx-a11y/no-noninteractive-tabindex -- the APG window-splitter pattern: a focusable separator with a value */}
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label={t(`layout.resize.${role}`)}
        aria-valuenow={width}
        aria-valuemin={limits.min}
        aria-valuemax={limits.max}
        tabIndex={0}
        className={cn(
          "group focus-visible:ring-ring absolute inset-y-0 z-10 w-1.5 cursor-col-resize touch-none outline-none focus-visible:ring-2",
          side === "left" ? "-right-0.75" : "-left-0.75",
        )}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        onDoubleClick={reset}
        onKeyDown={onKeyDown}
      >
        <span
          aria-hidden="true"
          className="group-hover:bg-foreground/30 group-active:bg-foreground/40 absolute inset-y-0 left-0.5 w-0.5 rounded-full transition-colors"
        />
      </div>
      {/* eslint-enable jsx-a11y/no-noninteractive-element-interactions, jsx-a11y/no-noninteractive-tabindex */}
    </Tag>
  );
}
