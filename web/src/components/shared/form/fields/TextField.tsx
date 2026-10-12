import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { TextControlProps } from "./types";
/** TextField edits a string through the shared input with accessible errors. */
export function TextField({
  id,
  label,
  value,
  onChange,
  invalid,
  describedBy,
  disabled,
  type = "text",
  ...props
}: TextControlProps) {
  return (
    <Input
      {...props}
      id={id}
      type={type}
      aria-label={label}
      value={value}
      disabled={disabled}
      aria-invalid={invalid || undefined}
      aria-describedby={describedBy}
      className={cn("h-9.5", invalid && "border-danger")}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}
