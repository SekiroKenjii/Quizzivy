import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import type { FieldControlProps } from "./types";
/** AreaField edits multiline text in the deck's three-row field. */
export function AreaField({
  id,
  label,
  value,
  onChange,
  invalid,
  describedBy,
  disabled,
  placeholder,
  maxLength,
}: FieldControlProps<string> &
  Readonly<{ placeholder?: string | undefined; maxLength?: number | undefined }>) {
  return (
    <Textarea
      id={id}
      aria-label={label}
      value={value}
      disabled={disabled}
      placeholder={placeholder}
      maxLength={maxLength}
      rows={3}
      aria-invalid={invalid || undefined}
      aria-describedby={describedBy}
      className={cn("px-3 py-2.25", invalid && "border-danger")}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}
