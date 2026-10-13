import { CircleAlert, Coins } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * AiCreditNote is the left of a panel's footer: what a call costs or cost and
 * what is left, beside the coins, or a sentence that stops the run, beside the
 * alert in the warning ink. It reads nothing itself; the panel gives it only
 * figures it has metered.
 */
export function AiCreditNote({
  text,
  warning,
}: Readonly<{ text: string; warning: boolean }>) {
  const Icon = warning ? CircleAlert : Coins;
  return (
    <p
      aria-live="polite"
      className={cn(
        "text-meta flex min-w-0 flex-[1_1_180px] items-center gap-1.5 leading-normal",
        warning ? "text-warning-ink" : "text-muted-fg",
      )}
    >
      <Icon aria-hidden="true" className="size-3.5 flex-none" />
      <span className="min-w-0">{text}</span>
    </p>
  );
}
