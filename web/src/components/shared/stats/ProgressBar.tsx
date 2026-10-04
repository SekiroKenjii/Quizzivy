import { cn } from "@/lib/utils";
import { percent } from "./progress";

type ProgressTone = "accent" | "danger" | "warning" | "success";
type ProgressSize = 6 | 8;

const FILLS: Record<ProgressTone, string> = {
  accent: "bg-brand",
  danger: "bg-danger",
  warning: "bg-warning",
  success: "bg-success",
};

const HEIGHTS: Record<ProgressSize, string> = {
  6: "h-1.5",
  8: "h-2",
};

const RADII: Record<ProgressSize, string> = {
  6: "rounded-[0.375rem]",
  8: "rounded-[0.5rem]",
};

/**
 * ProgressBarProps is what a ProgressBar takes: `value` of `max` (100 by
 * default), the `label` that names it, the track's height in pixels, the
 * fill's tone, and whether the fill ends square, clipped by the track, or
 * round.
 */
export type ProgressBarProps = Readonly<{
  value: number;
  max?: number | undefined;
  label: string;
  size?: ProgressSize | undefined;
  tone?: ProgressTone | undefined;
  cap?: "square" | "round" | undefined;
  className?: string | undefined;
}>;

/**
 * ProgressBar is the deck's thin bar of how much of something is done: a
 * muted track of 6 or 8 pixels and a fill as wide as `value` is of `max`, in
 * whole percent between 0 and 100. It is a progressbar named by `label`, and
 * the percentage is its value. It is built of spans, so it can sit inside a
 * link or a button; a caller gives it its width with `className`.
 */
export function ProgressBar({
  value,
  max = 100,
  label,
  size = 6,
  tone = "accent",
  cap = "square",
  className,
}: ProgressBarProps) {
  const now = percent(value, max);
  return (
    <span
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={now}
      className={cn(
        "bg-muted block overflow-hidden",
        HEIGHTS[size],
        RADII[size],
        className,
      )}
    >
      <span
        className={cn("block h-full", FILLS[tone], cap === "round" && RADII[size])}
        style={{ width: `${now}%` }}
      />
    </span>
  );
}
