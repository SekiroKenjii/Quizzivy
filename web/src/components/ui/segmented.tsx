import { useEffect, useRef } from "react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * SegmentedOption is one choice of a Segmented: its value, its label, an
 * optional icon before the label and an optional count after it. A count is
 * drawn whenever it is a number, zero included.
 */
export type SegmentedOption = {
  value: string;
  label: string;
  icon?: LucideIcon | undefined;
  count?: number | undefined;
};

type SegmentedSize = "default" | "lg" | "sm" | "xs";

const SIZES: Record<SegmentedSize, string> = {
  default: "",
  lg: "h-8 in-data-[scale=deck]:h-8",
  sm: "px-2.5 text-meta leading-4 in-data-[scale=deck]:h-7",
  xs: "h-6.5 gap-1.25 px-2.5 text-xs leading-[0.9375rem] in-data-[scale=deck]:h-6.5 in-data-[scale=deck]:rounded-sm",
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
  if (pressed === null) return;
  const frame = track.getBoundingClientRect();
  const box = pressed.getBoundingClientRect();
  if (box.left < frame.left) {
    track.scrollLeft -= frame.left - box.left + TRACK_PADDING;
  } else if (box.right > frame.right) {
    track.scrollLeft += box.right - frame.right + TRACK_PADDING;
  }
}

/**
 * Segmented is a group of buttons where exactly one is on: `role="group"`
 * with `aria-pressed` buttons, not a tab strip, because it controls no panel
 * (DG-17). On a `data-scale="deck"` surface the buttons are the deck's 30px
 * with the card fill and a 1px ring when on. `size="lg"` is the deck's 32px
 * row, used for a page's sections, `sm` its 28px row and `xs` the 26px mode
 * switch of an editor. An option's count is part of its button's name, as a
 * number. With `scroll` the track is at most as wide as its container and
 * scrolls sideways inside itself, without a scrollbar, bringing the pressed
 * option into view when it changes.
 */
export function Segmented({
  label,
  value,
  options,
  onChange,
  size = "default",
  scroll = false,
  className,
}: Readonly<{
  label: string;
  value: string;
  options: readonly SegmentedOption[];
  onChange: (value: string) => void;
  size?: SegmentedSize;
  scroll?: boolean;
  className?: string;
}>) {
  const track = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scroll && track.current !== null) revealPressed(track.current);
  }, [scroll, value]);

  return (
    <div
      ref={track}
      role="group"
      aria-label={label}
      className={cn(
        "bg-muted in-data-[scale=deck]:rounded-ctl inline-flex gap-0.5 rounded-lg p-[0.1875rem]",
        size === "xs" && "in-data-[scale=deck]:rounded-md",
        scroll &&
          "max-w-full [scrollbar-width:none] overflow-x-auto [&::-webkit-scrollbar]:hidden",
        className,
      )}
    >
      {options.map((option) => {
        const on = option.value === value;
        const Icon = option.icon;
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={on}
            onClick={() => onChange(option.value)}
            className={cn(
              "inline-flex h-7 items-center gap-1.5 rounded-md border-0 bg-transparent px-3 text-[0.8125rem] font-medium transition-colors",
              "in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:whitespace-nowrap",
              scroll && "shrink-0 leading-4 whitespace-nowrap outline-offset-1!",
              SIZES[size],
              Icon !== undefined && size === "lg" && "gap-1.75",
              on
                ? "bg-background text-foreground shadow-card in-data-[scale=deck]:bg-card in-data-[scale=deck]:text-fg in-data-[scale=deck]:ring-border in-data-[scale=deck]:ring-1"
                : "text-muted-foreground in-data-[scale=deck]:text-muted-fg",
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
                  {option.count}
                </span>
              </>
            )}
          </button>
        );
      })}
    </div>
  );
}
