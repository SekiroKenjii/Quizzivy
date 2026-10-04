import { RadioGroup } from "radix-ui";

import { cn } from "@/lib/utils";

/**
 * RadioCardOption is one card of a RadioCards group: its value, its label, an
 * optional line under the label, and whether it can be chosen.
 */
export type RadioCardOption = {
  value: string;
  label: string;
  hint?: string | undefined;
  disabled?: boolean | undefined;
};

/**
 * RadioCards is a radio group drawn as the deck's cards, each with a radio
 * circle, a label and a hint: one tab stop, arrow keys move the choice. The
 * cards sit in a grid of columns at least 240px wide, which a caller replaces
 * with `className`. `label` names the group.
 */
export function RadioCards({
  label,
  value,
  onChange,
  options,
  className,
}: Readonly<{
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: readonly RadioCardOption[];
  className?: string | undefined;
}>) {
  return (
    <RadioGroup.Root
      aria-label={label}
      value={value}
      onValueChange={onChange}
      className={cn(
        "grid grid-cols-[repeat(auto-fit,minmax(min(100%,240px),1fr))] gap-2.5",
        className,
      )}
    >
      {options.map((option) => (
        <RadioGroup.Item
          key={option.value}
          value={option.value}
          disabled={option.disabled === true}
          className="group bg-card border-border data-[state=checked]:border-primary flex items-start gap-3 rounded-lg border-[1.5px] px-3.5 py-3 text-left disabled:opacity-45"
        >
          <span
            aria-hidden="true"
            className="border-ring group-data-[state=checked]:border-primary mt-px grid size-4.5 flex-none place-items-center rounded-full border-2"
          >
            <span className="group-data-[state=checked]:bg-primary size-2 rounded-full" />
          </span>
          <span className="min-w-0">
            <span className="text-ui block leading-4.5 font-medium">
              {option.label}
            </span>
            {option.hint !== undefined && (
              <span className="text-muted-fg text-meta block">{option.hint}</span>
            )}
          </span>
        </RadioGroup.Item>
      ))}
    </RadioGroup.Root>
  );
}
