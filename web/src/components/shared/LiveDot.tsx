import { cn } from "@/lib/utils";

/**
 * LiveDot is the deck's pulsing "happening now" dot. It carries no meaning of
 * its own, so it is hidden from assistive technology: the text beside it says
 * what is live. It holds still under reduced motion.
 */
export function LiveDot({ className }: Readonly<{ className?: string }>) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "qz-live-dot bg-success inline-block size-[7px] shrink-0 rounded-full",
        className,
      )}
    />
  );
}
