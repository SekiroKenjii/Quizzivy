import { DateTimeField } from "@/components/shared/DateTimeField";
import type { FieldControlProps } from "./types";
/** DateField edits a wall-clock date through the shared picker. */
export function DateField({
  id,
  label,
  value,
  onChange,
  invalid,
  describedBy,
  minuteStep,
}: FieldControlProps<string> & Readonly<{ minuteStep?: number | undefined }>) {
  return (
    <div
      aria-invalid={invalid || undefined}
      aria-describedby={describedBy}
      className={invalid ? "[&_button]:border-danger" : undefined}
    >
      <DateTimeField
        id={id}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        label={label}
        value={value}
        onChange={onChange}
        mode="date"
        {...(minuteStep === undefined ? {} : { minuteStep })}
      />
    </div>
  );
}
