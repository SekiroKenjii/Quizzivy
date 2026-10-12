import { Check } from "lucide-react";
import { cn } from "@/lib/utils";
import type { FieldControlProps, FormOption } from "./types";
/** ChipsField toggles multiple values through pressed pill buttons. */
export function ChipsField({
  id,
  label,
  value,
  onChange,
  invalid: _invalid,
  describedBy,
  disabled,
  options,
}: FieldControlProps<readonly string[]> &
  Readonly<{ options: readonly FormOption[] }>) {
  const chosen = new Set(value);
  return (
    <div
      id={id}
      role="group"
      aria-label={label}
      aria-describedby={describedBy}
      className="flex flex-wrap gap-1.5"
    >
      {options.map((option) => {
        const on = chosen.has(option.value);
        return (
          <button
            key={option.value}
            type="button"
            disabled={disabled}
            aria-pressed={on}
            aria-describedby={describedBy}
            className={cn(
              "border-border bg-card text-meta inline-flex h-7.5 items-center gap-1.5 rounded-full border px-3 leading-normal font-medium",
              on && "border-primary bg-primary text-primary-fg",
            )}
            onClick={() =>
              onChange(
                on ? value.filter((v) => v !== option.value) : [...value, option.value],
              )
            }
          >
            {on && <Check aria-hidden="true" className="size-3.25" />}
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
