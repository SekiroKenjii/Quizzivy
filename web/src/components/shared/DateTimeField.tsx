import { useLayoutEffect, useRef, type AriaAttributes } from "react";
import { DateTimePicker } from "./DateTimePicker";

/** DateTimeMode chooses the wall-clock components edited by the field. */
export type DateTimeMode = "date" | "time" | "datetime";

interface DateTimeFieldProps {
  readonly id?: string;
  /** Names the halves for assistive tech; the label element names the first half through `id`. */
  readonly label: string;
  /** `yyyy-MM-ddTHH:mm`, `yyyy-MM-dd` or `HH:mm`, by mode; empty when nothing is chosen yet. */
  readonly value: string;
  readonly onChange: (next: string) => void;
  readonly mode?: DateTimeMode;
  /** Minutes offered in the clock; a value off the step is still shown and kept. */
  readonly minuteStep?: number;
  readonly className?: string;
  readonly disabled?: boolean | undefined;
  readonly "aria-invalid"?: AriaAttributes["aria-invalid"];
  readonly "aria-describedby"?: string | undefined;
}

/** DateTimeField edits wall-clock values with the deck's picker, which dismisses when disabled. */
export function DateTimeField({
  id,
  label,
  value,
  onChange,
  mode = "datetime",
  minuteStep = 5,
  className,
  disabled = false,
  "aria-invalid": invalid,
  "aria-describedby": describedBy,
}: DateTimeFieldProps) {
  const acceptsChanges = useRef(!disabled);
  useLayoutEffect(() => {
    acceptsChanges.current = !disabled;
    return () => {
      acceptsChanges.current = false;
    };
  }, [disabled]);
  return (
    <DateTimePicker
      id={id}
      label={label}
      value={value}
      onChange={(next) => {
        if (acceptsChanges.current) onChange(next);
      }}
      disabled={disabled}
      mode={mode}
      minuteStep={minuteStep}
      className={className}
      aria-invalid={invalid}
      aria-describedby={describedBy}
    />
  );
}
