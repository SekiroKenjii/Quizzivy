import { Switch } from "@/components/ui/switch";
import type { FieldControlProps } from "./types";
/** ToggleField edits a boolean with its visible text naming the switch. */
export function ToggleField({
  id,
  value,
  onChange,
  invalid,
  describedBy,
  disabled,
  text,
  sub,
}: FieldControlProps<boolean> & Readonly<{ text: string; sub?: string | undefined }>) {
  return (
    <div className="flex items-center gap-3">
      <label htmlFor={id} className="min-w-0 flex-1">
        <span className="text-ui block font-medium">{text}</span>
        {sub && <span className="text-muted-fg block text-xs">{sub}</span>}
      </label>
      <Switch
        id={id}
        checked={value}
        onCheckedChange={onChange}
        disabled={disabled ?? false}
        aria-label={text}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
      />
    </div>
  );
}
