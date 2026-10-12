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

/** ProgressBarProps supplies the labelled amount, maximum, track height, tone and cap shape. */
export type ProgressBarProps = Readonly<{
  value: number;
  max?: number | undefined;
  label: string;
  size?: ProgressSize | undefined;
  tone?: ProgressTone | undefined;
  cap?: "square" | "round" | undefined;
  className?: string | undefined;
}>;

/** ProgressBar shows a labelled percentage clamped between zero and one hundred. */
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
