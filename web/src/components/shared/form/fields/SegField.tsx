import { Segmented } from "@/components/ui/segmented";
import type { FieldControlProps, FormOption } from "./types";
/** SegField selects one value with equally sized wrapping form buttons. */
export function SegField({
  id,
  label,
  value,
  onChange,
  invalid,
  describedBy,
  options,
}: FieldControlProps<string> & Readonly<{ options: readonly FormOption[] }>) {
  return (
    <div id={id} aria-invalid={invalid || undefined} aria-describedby={describedBy}>
      <Segmented
        label={label}
        value={value}
        onChange={onChange}
        options={options}
        fill
      />
    </div>
  );
}
