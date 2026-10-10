import { Check } from "lucide-react";
import { useTranslation } from "react-i18next";
import { MarqueeText } from "@/components/shared/MarqueeText";
import { cn } from "@/lib/utils";

/**
 * WizardStep is one entry of the stepper: its name, the line that says what
 * it holds, and whether it can be opened yet.
 */
export interface WizardStep {
  label: string;
  value: string;
  disabled?: boolean;
}

/**
 * WizardStepper is the deck's row of numbered steps, four columns from 768px
 * and two below. `current` is the open step's index; the steps before it are
 * done and show a check. Each value line scrolls only when it overflows and
 * stays still under reduced motion.
 */
export function WizardStepper({
  steps,
  current,
  onOpen,
}: Readonly<{
  steps: readonly WizardStep[];
  current: number;
  onOpen: (index: number) => void;
}>) {
  const { t } = useTranslation();
  return (
    <ol
      aria-label={t("assignments.wizard.stepsLabel")}
      className="m-0 grid list-none grid-cols-2 gap-2 p-0 min-[768px]:grid-cols-4"
    >
      {steps.map((step, index) => {
        const on = index === current;
        const done = index < current;
        return (
          <li key={step.label} className="min-w-0">
            <button
              type="button"
              aria-current={on ? "step" : undefined}
              disabled={step.disabled === true}
              onClick={() => onOpen(index)}
              className={cn(
                "bg-card flex w-full items-center gap-2.5 rounded-[10px] border px-3 py-2.5 text-left disabled:cursor-not-allowed disabled:opacity-60",
                on ? "border-primary" : "border-border",
              )}
            >
              <span
                aria-hidden="true"
                className={cn(
                  "grid size-6 flex-none place-items-center rounded-full text-xs font-semibold",
                  on || done ? "bg-primary text-primary-fg" : "bg-muted text-muted-fg",
                )}
              >
                {done ? <Check className="size-3.5" strokeWidth={2.5} /> : index + 1}
              </span>
              <span className="min-w-0 flex-1 overflow-hidden">
                <span className="text-ui block truncate leading-[18px] font-medium">
                  {step.label}
                  {done ? (
                    <span className="sr-only">{t("assignments.wizard.stepDone")}</span>
                  ) : null}
                </span>
                <MarqueeText
                  text={step.value}
                  minSeconds={9}
                  maskPx={12}
                  className="text-muted-fg text-xs leading-[15px]"
                />
              </span>
            </button>
          </li>
        );
      })}
    </ol>
  );
}
