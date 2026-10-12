import { cn } from "@/lib/utils";
import { waveformBars, waveformSeed } from "./waveformBars";

/** Waveform renders deterministic decorative bars with a clamped progress fraction. */
export function Waveform({
  seed,
  progress = 0,
  className,
}: Readonly<{ seed: string | number; progress?: number; className?: string }>) {
  const played = Math.round(
    Math.max(0, Math.min(1, Number.isNaN(progress) ? 0 : progress)) * 28,
  );
  return (
    <span
      aria-hidden="true"
      className={cn("flex h-11 min-w-0 flex-1 items-center gap-0.5", className)}
    >
      {waveformBars(waveformSeed(seed)).map((height, index) => (
        <span
          key={index}
          style={{ height: `${height}%` }}
          className={cn("flex-1 rounded-[2px]", index < played ? "bg-fg" : "bg-ring")}
        />
      ))}
    </span>
  );
}
