import { useEffect, useMemo, useRef, type FocusEvent } from "react";
import type { LucideIcon } from "lucide-react";

import { useLocale } from "@/lib/i18n/useLocale";
import { revealWithin } from "@/lib/revealWithin";
import { cn } from "@/lib/utils";

/**
 * SegmentedOption is one choice of a Segmented: its value, its label, an
 * optional icon before the label and an optional count after it. A count is
 * drawn whenever it is a number, zero included, grouped for the viewer's
 * language ("1.248"). A disabled option cannot be
 * chosen; `describedBy` names the element that says why. A pending option is
 * unavailable while a request it started runs: it answers `aria-disabled`,
 * ignores a press and keeps focus (T-35).
 */
export type SegmentedOption = {
  value: string;
  label: string;
  icon?: LucideIcon | undefined;
  count?: number | undefined;
  disabled?: boolean | undefined;
  pending?: boolean | undefined;
  describedBy?: string | undefined;
};

type SegmentedSize = "default" | "lg" | "sm" | "xs";

const SIZES: Record<SegmentedSize, string> = {
  default: "",
  lg: "h-8",
  sm: "px-2.5 text-meta leading-4 h-7",
  xs: "gap-1.25 px-2.5 text-xs leading-[0.9375rem] h-6.5 rounded-sm",
};

const ICONS: Record<SegmentedSize, string> = {
  default: "size-[0.9375rem]",
  lg: "size-[0.9375rem]",
  sm: "size-[0.8125rem]",
  xs: "size-[0.8125rem]",
};

const TRACK_PADDING = 3;

function revealPressed(track: HTMLElement) {
  const pressed = track.querySelector('[aria-pressed="true"]');
  if (pressed !== null) revealWithin(track, pressed, TRACK_PADDING);
}

function revealFocused(event: FocusEvent<HTMLDivElement>) {
  revealWithin(event.currentTarget, event.target, TRACK_PADDING);
}

/**
 * Segmented selects one grouped option, with optional scrolling or
 * equal-width wrapping form buttons whose labels sit centred: 30px high, or
 * 32px with 13px labels at size `lg`, as the deck draws the assignment
 * wizard's rule groups. While `pending`, every option answers `aria-disabled`
 * and ignores a press, so the focused one keeps focus (T-35); the caller
 * decides when a request makes the control unavailable.
 */
export function Segmented({
  label,
  value,
  options,
  onChange,
  size = "default",
  scroll = false,
  fill = false,
  pending = false,
  className,
}: Readonly<{
  label: string;
  value: string;
  options: readonly SegmentedOption[];
  onChange: (value: string) => void;
  size?: SegmentedSize;
  scroll?: boolean;
  fill?: boolean;
  pending?: boolean;
  className?: string;
}>) {
  const track = useRef<HTMLDivElement>(null);
  const locale = useLocale();
  const figure = useMemo(() => new Intl.NumberFormat(locale), [locale]);

  useEffect(() => {
    if (scroll && track.current !== null) revealPressed(track.current);
  }, [scroll, value]);

  return (
    <div
      ref={track}
      role="group"
      aria-label={label}
      onFocus={scroll ? revealFocused : undefined}
      className={cn(
        "bg-muted rounded-ctl inline-flex gap-0.5 p-[0.1875rem]",
        fill && "flex w-full flex-wrap",
        size === "xs" && "rounded-md",
        scroll &&
          "max-w-full [scrollbar-width:none] overflow-x-auto [&::-webkit-scrollbar]:hidden",
        className,
      )}
    >
      {options.map((option) => {
        const on = option.value === value;
        const unavailable = pending || option.pending === true;
        const Icon = option.icon;
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={on}
            disabled={option.disabled}
            aria-disabled={unavailable || undefined}
            aria-describedby={option.describedBy}
            onClick={() => {
              if (!unavailable) onChange(option.value);
            }}
            className={cn(
              "inline-flex items-center gap-1.5 border-0 bg-transparent px-3 text-[0.8125rem] font-medium transition-colors",
              "rounded-seg h-7.5 whitespace-nowrap",
              scroll && "shrink-0 leading-4 whitespace-nowrap outline-offset-1!",
              SIZES[size],
              fill && "flex-1 justify-center leading-normal",
              fill && size !== "lg" && "text-meta h-7.5 px-2.5",
              Icon !== undefined && size === "lg" && "gap-1.75",
              on ? "shadow-card bg-card text-fg ring-border ring-1" : "text-muted-fg",
              (option.disabled || unavailable) && "text-muted-fg/45 cursor-not-allowed",
            )}
          >
            {Icon !== undefined && (
              <Icon aria-hidden="true" className={cn("shrink-0", ICONS[size])} />
            )}
            {option.label}
            {typeof option.count === "number" && (
              <>
                {" "}
                <span className="bg-primary text-primary-fg text-2xs inline-flex h-4.5 min-w-4.5 items-center justify-center rounded-full px-1.25 leading-none font-semibold tabular-nums">
                  {figure.format(option.count)}
                </span>
              </>
            )}
          </button>
        );
      })}
    </div>
  );
}
