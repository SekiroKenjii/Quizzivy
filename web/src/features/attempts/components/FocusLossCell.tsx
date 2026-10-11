import { Flag } from "lucide-react";
import { FLAGGED, focusLossLabel } from "@/features/integrity/tones";
import { cn } from "@/lib/utils";

/** FocusLossCell is the roster's "Focus lost" value: "—" or "{n}×", in danger ink with a flag once the attempt is flagged. */
export function FocusLossCell({
  count,
  flagged,
  compact = false,
}: Readonly<{
  count: number | null | undefined;
  flagged: boolean | undefined;
  compact?: boolean;
}>) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center whitespace-nowrap tabular-nums",
        compact ? "gap-0.5" : "gap-1.25",
        flagged ? FLAGGED.ink : "text-muted-fg",
      )}
    >
      {flagged && (
        <Flag aria-hidden="true" className={compact ? "size-3" : "size-3.25"} />
      )}
      {focusLossLabel(count)}
    </span>
  );
}
