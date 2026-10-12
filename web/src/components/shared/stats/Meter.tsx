import { cn } from "@/lib/utils";
import { shares } from "./progress";

type MeterTone = "accent" | "info" | "warning" | "danger";

const FILLS: Record<MeterTone, string> = {
  accent: "bg-brand",
  info: "bg-info",
  warning: "bg-warning",
  danger: "bg-danger",
};

/** MeterPart describes one ordered share of the gauge and its tone. */
export type MeterPart = Readonly<{
  key: string;
  value: number;
  tone: MeterTone;
}>;

/** MeterProps supplies the gauge label, accessible value text, maximum and ordered shares. */
export type MeterProps = Readonly<{
  label: string;
  valueText: string;
  max: number;
  parts: readonly MeterPart[];
  className?: string | undefined;
}>;

/** Meter draws ordered shares and caps the reported total at its maximum. */
export function Meter({ label, valueText, max, parts, className }: MeterProps) {
  const widths = shares(
    parts.map((part) => part.value),
    max,
  );
  const used = parts.reduce((sum, part) => sum + (part.value > 0 ? part.value : 0), 0);
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
