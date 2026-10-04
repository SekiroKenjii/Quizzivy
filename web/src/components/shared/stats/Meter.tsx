import { cn } from "@/lib/utils";
import { shares } from "./progress";

type MeterTone = "accent" | "info" | "warning" | "danger";

const FILLS: Record<MeterTone, string> = {
  accent: "bg-brand",
  info: "bg-info",
  warning: "bg-warning",
  danger: "bg-danger",
};

/**
 * MeterPart is one stretch of a Meter's fill: how much of the whole it takes,
 * in the unit `max` is in, and its tone.
 */
export type MeterPart = Readonly<{
  key: string;
  value: number;
  tone: MeterTone;
}>;

/**
 * MeterProps is what a Meter takes: the `label` that names it, the sentence
 * a screen reader hears as its value, the size of the whole and the parts
 * that fill it, in order.
 */
export type MeterProps = Readonly<{
  label: string;
  valueText: string;
  max: number;
  parts: readonly MeterPart[];
  className?: string | undefined;
}>;

/**
 * Meter is the deck's 8px gauge of how much of a fixed amount is used, the
 * parts drawn one after another from the start of the track. It is a meter
 * named by `label` whose value is the sum of its parts, never more than
 * `max`, and whose text is `valueText`. A part's width is its share of `max`
 * and a part that would run past the end is cut there. It is built of spans;
 * a caller gives it its width with `className`.
 */
export function Meter({ label, valueText, max, parts, className }: MeterProps) {
  const widths = shares(
    parts.map((part) => part.value),
    max,
  );
  const used = parts.reduce((sum, part) => sum + Math.max(0, part.value), 0);
  return (
    <span
      role="meter"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={max}
      aria-valuenow={Math.min(used, Math.max(0, max))}
      aria-valuetext={valueText}
      className={cn("bg-muted flex h-2 overflow-hidden rounded-[0.5rem]", className)}
    >
      {parts.map((part, index) => (
        <span
          key={part.key}
          className={FILLS[part.tone]}
          style={{ width: `${widths[index] ?? 0}%` }}
        />
      ))}
    </span>
  );
}
