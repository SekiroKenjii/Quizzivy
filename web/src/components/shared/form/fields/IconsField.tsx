import { cn } from "@/lib/utils";
import type { FieldControlProps, FormIconOption } from "./types";

/**
 * IconsField selects one icon from a grid of square buttons, 40px high and as
 * many to a row as the dialog's width holds. The choice is a radio group, so
 * the arrows move through it and a screen reader hears each icon's label; the
 * chosen one is filled with the primary colour. While `disabled`, every icon
 * is.
 */
export function IconsField({
  id,
  label,
  value,
  onChange,
  invalid,
  describedBy,
  disabled = false,
  options,
}: FieldControlProps<string> & Readonly<{ options: readonly FormIconOption[] }>) {
  return (
    <div
      id={id}
      role="radiogroup"
      aria-label={label}
      aria-invalid={invalid || undefined}
      aria-describedby={describedBy}
      className="grid grid-cols-[repeat(auto-fill,minmax(2.5rem,1fr))] gap-1.5"
    >
      {options.map((option) => {
        const Icon = option.icon;
        const on = option.value === value;
        return (
          <label
            key={option.value}
            title={option.label}
            className={cn(
              "has-[:focus-visible]:outline-focus grid h-10 cursor-pointer place-items-center rounded-[0.5625rem] border-[1.5px] transition-colors has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2",
              on
                ? "border-primary bg-primary text-primary-fg"
                : "border-border bg-card text-fg hover:bg-muted",
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
            <Icon aria-hidden="true" className="size-4.5" />
          </label>
        );
      })}
    </div>
  );
}
