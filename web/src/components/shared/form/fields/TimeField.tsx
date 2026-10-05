import { DateTimeField } from "@/components/shared/DateTimeField";
import type { FieldControlProps } from "./types";
/** TimeField edits a wall-clock time through the shared picker. */
export function TimeField({
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
        mode="time"
        {...(minuteStep === undefined ? {} : { minuteStep })}
      />
    </div>
  );
}
