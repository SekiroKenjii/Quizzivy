import { Check } from "lucide-react";
import { cn } from "@/lib/utils";
import type { FieldControlProps, FormColorOption } from "./types";

/**
 * ColorsField selects one colour from a row of 36px swatches. The choice is a
 * radio group, so the arrows move through it and a screen reader hears each
 * colour's label; the chosen swatch carries a tick and a 2px ring in the
 * primary colour. While `disabled`, every swatch is.
 */
export function ColorsField({
  id,
  label,
  value,
  onChange,
  invalid,
  describedBy,
  disabled = false,
  options,
}: FieldControlProps<string> & Readonly<{ options: readonly FormColorOption[] }>) {
  return (
    <div
      id={id}
      role="radiogroup"
      aria-label={label}
      aria-invalid={invalid || undefined}
      aria-describedby={describedBy}
      className="flex flex-wrap gap-2"
    >
      {options.map((option) => {
        const on = option.value === value;
        return (
          <label
            key={option.value}
            title={option.label}
            className={cn(
              "has-[:focus-visible]:outline-focus grid size-9 cursor-pointer place-items-center rounded-full border has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-4",
              option.swatch,
              on && "shadow-[0_0_0_2px_var(--card),0_0_0_4px_var(--primary)]",
              disabled && "cursor-not-allowed",
            )}
          >
            <input
              type="radio"
              name={id}
              value={option.value}
              checked={on}
              disabled={disabled}
              aria-label={option.label}
              className="sr-only"
              onChange={() => onChange(option.value)}
            />
            {on && <Check aria-hidden="true" className="size-3.75" />}
          </label>
        );
      })}
    </div>
  );
}
