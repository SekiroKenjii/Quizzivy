import { Segmented } from "@/components/ui/segmented";
import type { FieldControlProps, FormOption } from "./types";
/**
 * SegField selects one value with equally sized wrapping form buttons; while
 * `disabled`, every option is.
 */
export function SegField({
  id,
  label,
  value,
  onChange,
  invalid,
  describedBy,
  disabled = false,
  options,
}: FieldControlProps<string> & Readonly<{ options: readonly FormOption[] }>) {
  return (
    <div id={id} aria-invalid={invalid || undefined} aria-describedby={describedBy}>
      <Segmented
        label={label}
        value={value}
        onChange={onChange}
        options={
          disabled ? options.map((option) => ({ ...option, disabled: true })) : options
        }
        fill
      />
    </div>
  );
}
