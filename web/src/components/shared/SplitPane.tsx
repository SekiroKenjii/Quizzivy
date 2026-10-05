/* eslint-disable jsx-a11y/no-noninteractive-element-interactions, jsx-a11y/no-noninteractive-tabindex -- The APG splitter uses a focusable separator with a value. */
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import {
  clampSplitSize,
  nextSplitSize,
  percentSplitSize,
  pixelSplitSize,
  readSplitSize,
  writeSplitSize,
  type SplitUnit,
} from "./splitSize";

interface SplitPaneProps {
  label: string;
  first: ReactNode;
  second: ReactNode;
  unit: SplitUnit;
  defaultSize: number;
  min: number;
  max: number;
  minSecond?: number;
  step?: number;
  storageKey?: string;
  handle?: "grip" | "line";
  split?: boolean;
  className?: string;
  firstClassName?: string;
  secondClassName?: string;
}

/** SplitPane resizes stable pane wrappers in pixels or percentages without remounting their contents when stacked. */
export function SplitPane({
  label,
  first,
  second,
  unit,
  defaultSize,
  min,
  max,
  minSecond = 0,
  step = unit === "px" ? 16 : 2,
  storageKey,
  handle = "grip",
  split = true,
  className,
  firstClassName,
  secondClassName,
}: Readonly<SplitPaneProps>) {
  const { t } = useTranslation();
  const row = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState(() =>
    readSplitSize(storageKey, defaultSize, min, max),
  );
  const heldSize = useRef(size);
  const [measuredWidth, setMeasuredWidth] = useState<number | null>(null);
  const [dragging, setDragging] = useState(false);
  const drag = useRef<{
    pointerId: number;
    element: HTMLDivElement;
    x: number;
    size: number;
    left: number;
    width: number;
    ceiling: number;
    cursor: string;
    selection: string;
  } | null>(null);
  const gutter = handle === "grip" ? 14 : 0;
  const ceilingFor = (width: number | null) =>
    unit === "px" && width !== null
      ? Math.max(min, Math.min(max, width - gutter - minSecond))
      : Math.max(min, max);
  const ceiling = ceilingFor(measuredWidth);
  const bounded = clampSplitSize(size, min, max);
  const splitFlex =
    unit === "percent"
      ? `0 0 ${bounded}%`
      : `0 0 clamp(${min}px, ${bounded}px, calc(100% - ${gutter}px - ${minSecond}px))`;
  const stopDrag = useCallback(() => {
    const current = drag.current;
    if (current === null) return null;
    drag.current = null;
    document.body.style.cursor = current.cursor;
    document.body.style.userSelect = current.selection;
    try {
      current.element.releasePointerCapture(current.pointerId);
    } catch {
      return current;
    }
    return current;
  }, []);
  useEffect(
    () => () => {
      stopDrag();
      setDragging(false);
    },
    [split, stopDrag],
  );
  const apply = (next: number) => {
    heldSize.current = next;
    setSize(next);
  };
  const measure = () => {
    const rect = row.current?.getBoundingClientRect();
    const width = rect && rect.width > 0 ? rect.width : null;
    setMeasuredWidth(width);
    return { left: rect?.left ?? 0, width: width ?? 0, ceiling: ceilingFor(width) };
  };
  return (
    <div
      ref={row}
      className={cn("flex min-h-0 min-w-0", !split && "flex-wrap", className)}
    >
      <div
        key="first"
        className={cn("min-h-0 min-w-0", firstClassName)}
        style={{
          flex: split ? splitFlex : "1 1 100%",
        }}
      >
        {first}
      </div>
      {split && (
        <div
          key="separator"
          role="separator"
          aria-orientation="vertical"
          aria-label={label}
          aria-valuenow={Math.round(Math.min(bounded, ceiling))}
          aria-valuemin={min}
          aria-valuemax={ceiling}
          tabIndex={0}
          title={handle === "grip" ? t("splitPane.hint") : undefined}
          data-dragging={dragging || undefined}
          className={cn(
            "group relative z-10 flex shrink-0 cursor-col-resize touch-none justify-center self-stretch select-none",
            handle === "grip" ? "w-3.5" : "focus:bg-hover -mr-[5px] -ml-1 w-[9px]",
          )}
          onFocus={measure}
          onPointerDown={(event) => {
            if (event.button !== 0 || drag.current !== null) return;
            event.preventDefault();
            const bounds = measure();
            const current = clampSplitSize(heldSize.current, min, bounds.ceiling);
            apply(current);
            event.currentTarget.setPointerCapture(event.pointerId);
            drag.current = {
              pointerId: event.pointerId,
              element: event.currentTarget,
              x: event.clientX,
              size: current,
              ...bounds,
              cursor: document.body.style.cursor,
              selection: document.body.style.userSelect,
            };
            document.body.style.cursor = "col-resize";
            document.body.style.userSelect = "none";
            setDragging(true);
          }}
          onPointerMove={(event) => {
            const current = drag.current;
            if (current === null || current.pointerId !== event.pointerId) return;
            apply(
              unit === "px"
                ? pixelSplitSize(
                    current.size,
                    current.x,
                    event.clientX,
                    min,
                    current.ceiling,
                  )
                : percentSplitSize(
                    event.clientX,
                    current.left,
                    current.width,
                    min,
                    current.ceiling,
                  ),
            );
          }}
          onPointerUp={(event) => {
            if (drag.current?.pointerId !== event.pointerId) return;
            stopDrag();
            setDragging(false);
            writeSplitSize(storageKey, heldSize.current);
          }}
          onPointerCancel={(event) => {
            if (drag.current?.pointerId === event.pointerId) {
              stopDrag();
              setDragging(false);
            }
          }}
          onLostPointerCapture={() => {
            if (stopDrag()) setDragging(false);
          }}
          onKeyDown={(event) => {
            const bounds = measure();
            const next = nextSplitSize(
              event.key,
              clampSplitSize(heldSize.current, min, bounds.ceiling),
              step,
              min,
              bounds.ceiling,
            );
            if (next === null) return;
            event.preventDefault();
            apply(next);
            writeSplitSize(storageKey, next);
          }}
          onDoubleClick={() => {
            measure();
            stopDrag();
            setDragging(false);
            apply(clampSplitSize(defaultSize, min, max));
            writeSplitSize(storageKey, null);
          }}
        >
          <span
            aria-hidden="true"
            className={
              handle === "grip"
                ? "bg-border group-hover:bg-ring group-data-[dragging=true]:bg-fg sticky top-[calc(50%-20px)] mt-[180px] h-10 w-1 rounded-[4px] transition-[background-color] duration-150 ease-[cubic-bezier(.25,.1,.25,1)] motion-reduce:transition-none"
                : "bg-border group-data-[dragging=true]:bg-ring h-full w-px"
            }
          />
        </div>
      )}
      <div
        key="second"
        className={cn("min-h-0 min-w-0", secondClassName)}
        style={{ flex: split ? "1 1 0" : "1 1 100%" }}
      >
        {second}
      </div>
    </div>
  );
}
