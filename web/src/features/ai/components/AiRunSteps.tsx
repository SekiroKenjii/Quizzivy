import { useEffect, useState } from "react";
import { Check, LoaderCircle } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

const STEP_MS = 1300;
const SKELETON_HEIGHTS = ["h-23", "h-30", "h-23"] as const;

/**
 * AiRunSteps is a panel's running stage: its step lines, done, under way or
 * waiting, the next one taken every 1.3 s until the last, then a pulsing
 * skeleton of the cards to come. The pulse and the spinner are still under
 * reduced motion.
 */
export function AiRunSteps({ steps }: Readonly<{ steps: readonly string[] }>) {
  const [step, setStep] = useState(0);
  const last = steps.length - 1;
  useEffect(() => {
    const timer = window.setInterval(
      () => setStep((current) => Math.min(last, current + 1)),
      STEP_MS,
    );
    return () => window.clearInterval(timer);
  }, [last]);
  return (
    <div role="status" aria-live="polite" className="flex flex-col gap-4.5">
      <ol className="flex flex-col gap-2.5">
        {steps.map((label, index) => (
          <li
            key={label}
            aria-current={index === step ? "step" : undefined}
            className={cn(
              "text-ui flex items-center gap-2.5 leading-normal",
              index <= step ? "text-fg" : "text-muted-fg",
            )}
          >
            <span
              aria-hidden="true"
              className="grid size-5 flex-none place-items-center"
            >
              {index < step && <Check className="text-success size-3.75" />}
              {index === step && (
                <LoaderCircle className="size-3.5 animate-spin motion-reduce:animate-none" />
              )}
              {index > step && <span className="bg-border size-1.75 rounded-full" />}
            </span>
            {label}
          </li>
        ))}
      </ol>
      <div className="flex flex-col gap-2.5">
        {SKELETON_HEIGHTS.map((height, index) => (
          <Skeleton
            key={index}
            className={cn("rounded-xl motion-reduce:animate-none", height)}
          />
        ))}
      </div>
    </div>
  );
}
